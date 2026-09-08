#!/bin/bash
# Merge size artifacts, query DynamoDB baselines, compute deltas, post PR comment.
#
# Required env: GH_TOKEN, PR_NUMBER
# Usage: report-binary-sizes.sh <reports-dir>

set -euo pipefail

REPORTS_DIR="${1:-.}"
PR_NUMBER="${PR_NUMBER:?PR_NUMBER is required}"
TABLE_NAME="CWABinarySizes"
REGION="us-west-2"
MARKER="<!-- binary-size-report -->"

# Check if there are any reports to process
if ! ls "$REPORTS_DIR"/size-report-*.json &>/dev/null; then
     echo "No size report artifacts found."
     exit 0
fi

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

echo "Querying DynamoDB for recent main commits..."
aws dynamodb query \
     --table-name "$TABLE_NAME" \
     --index-name BranchDateIndex \
     --key-condition-expression "Branch = :b" \
     --expression-attribute-values '{":b": {"S": "main"}}' \
     --no-scan-index-forward \
     --max-items 9 \
     --region "$REGION" \
     --output json >"$WORK_DIR/main_baseline.json" 2>/dev/null || echo '{"Items":[]}' >"$WORK_DIR/main_baseline.json"

echo "Querying DynamoDB for latest release baseline..."
aws dynamodb query \
     --table-name "$TABLE_NAME" \
     --index-name ReleaseDateIndex \
     --key-condition-expression "RecordType = :rt" \
     --expression-attribute-values '{":rt": {"S": "release"}}' \
     --no-scan-index-forward \
     --max-items 1 \
     --region "$REGION" \
     --output json >"$WORK_DIR/release_baseline.json" 2>/dev/null || echo '{"Items":[]}' >"$WORK_DIR/release_baseline.json"

echo "Generating report..."
COMMENT_BODY=$(
     python3 - "$REPORTS_DIR" "$WORK_DIR/main_baseline.json" "$WORK_DIR/release_baseline.json" <<'EOF'
import json, glob, os, sys

reports_dir, main_path, release_path = sys.argv[1], sys.argv[2], sys.argv[3]

# Merge all runner artifacts
files = glob.glob(os.path.join(reports_dir, "size-report-*.json"))
if not files:
    exit(0)

pr_sizes = {}
for f in files:
    with open(f) as fh:
        data = json.load(fh)
    for key, size in data.get("binaries", {}).items():
        pr_sizes[key] = size

if not pr_sizes:
    exit(0)

# Parse DynamoDB baselines
def parse_dynamo_items(path):
    with open(path) as f:
        data = json.load(f)
    return data.get("Items", [])

def parse_item(item):
    commit = item.get("CommitHash", {}).get("S", "")
    tag = item.get("Tag", {}).get("S", "")
    binaries = {}
    for key, val in item.get("Binaries", {}).get("M", {}).items():
        binaries[key] = int(val.get("N", 0))
    return binaries, commit, tag

main_items = parse_dynamo_items(main_path)
release_items = parse_dynamo_items(release_path)

main_sizes, main_commit, _ = parse_item(main_items[0]) if main_items else ({}, "", "")
release_sizes, release_commit, release_tag = parse_item(release_items[0]) if release_items else ({}, "", "")

# Formatting helpers
def fmt_size(b):
    if b >= 1_000_000:
        return f"{b / 1_000_000:.1f} MB"
    elif b >= 1_000:
        return f"{b / 1_000:.1f} KB"
    return f"{b} B"

def fmt_delta(delta, base):
    if delta == 0:
        return "+0 B"
    if base and base > 0:
        pct = delta / base * 100
        sign = "+" if delta > 0 else ""
        indicator = r"${\color{red}▲}$ " if delta > 0 else r"${\color{green}▼}$ "
        return f"{indicator}{sign}{fmt_size(abs(delta))} ({sign}{pct:.1f}%)"
    sign = "+" if delta > 0 else ""
    indicator = r"${\color{red}▲}$ " if delta > 0 else r"${\color{green}▼}$ "
    return f"{indicator}{sign}{fmt_size(abs(delta))}"

def sort_key(key):
    parts = key.split("/")
    if len(parts) == 2:
        platform, binary = parts
    else:
        platform, binary = "", key
    base_binary = binary.removesuffix(".exe")
    return (base_binary, platform)

# Group binaries by platform
from collections import defaultdict
platforms = defaultdict(list)
for key in sorted(pr_sizes.keys(), key=sort_key):
    parts = key.split("/")
    if len(parts) == 2:
        platform = parts[0]
        binary = parts[1]
    else:
        platform = "other"
        binary = key
    platforms[platform].append((key, binary))

# Preferred display order, primary platform first
platform_order = ["linux_amd64", "linux_arm64", "windows_amd64"]
ordered_platforms = [p for p in platform_order if p in platforms]
ordered_platforms.extend(p for p in platforms if p not in platform_order)

def build_table(platform, entries, main_sizes, release_sizes):
    rows = []
    total_pr = 0
    total_main_base = 0
    total_release_base = 0
    total_main_delta = 0
    total_release_delta = 0
    has_main = False
    has_release = False

    for key, binary in entries:
        pr_size = pr_sizes[key]
        total_pr += pr_size
        pr_col = fmt_size(pr_size)
        main_col = "-"
        release_col = "-"

        if key in main_sizes:
            has_main = True
            delta = pr_size - main_sizes[key]
            total_main_delta += delta
            total_main_base += main_sizes[key]
            main_col = fmt_delta(delta, main_sizes[key])

        if key in release_sizes:
            has_release = True
            delta = pr_size - release_sizes[key]
            total_release_delta += delta
            total_release_base += release_sizes[key]
            release_col = fmt_delta(delta, release_sizes[key])

        rows.append(f"| {binary} | {pr_col} | {main_col} | {release_col} |")

    total_main_col = "-"
    total_release_col = "-"
    if has_main:
        total_main_col = f"**{fmt_delta(total_main_delta, total_main_base)}**"
    if has_release:
        total_release_col = f"**{fmt_delta(total_release_delta, total_release_base)}**"
    rows.append(f"| **Total** | **{fmt_size(total_pr)}** | {total_main_col} | {total_release_col} |")

    return rows

# Build markdown
repo = "aws/amazon-cloudwatch-agent"
marker = "<!-- binary-size-report -->"

main_header = "vs Main"
if main_commit:
    sha = main_commit[:7]
    main_header = f"vs `main` ([`{sha}`](https://github.com/{repo}/commit/{sha}))"

release_header = "vs Release"
if release_tag:
    release_header = f"vs [`{release_tag}`](https://github.com/{repo}/releases/tag/{release_tag})"

table_header = f"| Binary | PR | {main_header} | {release_header} |"
table_sep = "|--------|---:|--------:|-----------:|"

lines = [
    marker,
    "## Binary Size Report",
    "",
]

primary = ordered_platforms[0] if ordered_platforms else None
if primary:
    lines.append(f"### {primary.replace('_', '/')}")
    lines.append("")
    lines.append(table_header)
    lines.append(table_sep)
    lines.extend(build_table(primary, platforms[primary], main_sizes, release_sizes))
    lines.append("")

# Trend graph for linux/amd64 agent binary
trend_key = "linux_amd64/amazon-cloudwatch-agent"
if main_items and trend_key in pr_sizes:
    trend_points = []
    for item in reversed(main_items):
        binaries, commit, tag = parse_item(item)
        size = binaries.get(trend_key, 0)
        if size > 0:
            label = tag if tag else commit[:7]
            trend_points.append((label, size, bool(tag), commit))

    # Add PR as the rightmost point
    trend_points.append(("PR", pr_sizes[trend_key], False, ""))

    if len(trend_points) >= 3:
        sizes = [p[1] for p in trend_points]
        min_s = min(sizes)
        max_s = max(sizes)
        range_s = max_s - min_s
        pad = range_s * 0.05 if range_s > 0 else 1_000_000
        min_s -= pad
        max_s += pad
        height = 8
        bar_width = 3
        gap = 1
        cell = bar_width + gap

        graph_lines = []
        graph_lines.append(f"linux/amd64 amazon-cloudwatch-agent (last {len(trend_points) - 1} main commits + this PR)")
        graph_lines.append("")
        for row in range(height, -1, -1):
            if row == height:
                label = f"{max_s/1_000_000:.0f}"
            elif row == 0:
                label = f"{min_s/1_000_000:.0f}"
            else:
                label = ""
            line = f"{label:>4} ┤"
            for i, (_, size, _, _) in enumerate(trend_points):
                normalized = (size - min_s) / (max_s - min_s) * height
                if normalized >= row + 0.5:
                    line += "█" * bar_width + " " * gap
                elif normalized >= row:
                    line += "▄" * bar_width + " " * gap
                else:
                    line += " " * cell
            graph_lines.append(line)

        graph_lines.append(f" MB  └" + "─" * (cell * len(trend_points)))

        # X-axis labels: show tags and "PR" marker
        axis_labels = []
        for i, (label, _, is_tag, _) in enumerate(trend_points):
            if is_tag or label == "PR":
                axis_labels.append((i, label))
        # Always show first and last
        if not any(i == 0 for i, _ in axis_labels):
            axis_labels.insert(0, (0, trend_points[0][0]))
        if not any(i == len(trend_points) - 1 for i, _ in axis_labels):
            axis_labels.append((len(trend_points) - 1, trend_points[-1][0]))

        axis = list(" " * (6 + cell * len(trend_points)))
        for idx, lbl in sorted(axis_labels):
            pos = 6 + idx * cell
            for j, c in enumerate(lbl):
                if pos + j < len(axis):
                    axis[pos + j] = c
        graph_lines.append("".join(axis).rstrip())

        # Notable changes (outside code block for links)
        jumps = []
        for i in range(1, len(trend_points)):
            delta = trend_points[i][1] - trend_points[i-1][1]
            if abs(delta) > 1_000_000:
                sign = "+" if delta > 0 else ""
                lbl, _, is_tag, full_commit = trend_points[i]
                if is_tag:
                    link = f"[`{lbl}`](https://github.com/{repo}/releases/tag/{lbl})"
                elif full_commit:
                    link = f"[`{lbl}`](https://github.com/{repo}/commit/{full_commit})"
                else:
                    link = f"`{lbl}`"
                jumps.append(f"- {sign}{delta/1_000_000:.1f} MB at {link}")

        lines.append("")
        lines.append("```")
        lines.extend(graph_lines)
        lines.append("```")
        if jumps:
            lines.append("")
            lines.append("Notable changes:")
            lines.extend(jumps)
        lines.append("")

for platform in ordered_platforms[1:]:
    display_name = platform.replace("_", "/")
    lines.append(f"<details>")
    lines.append(f"<summary>{display_name}</summary>")
    lines.append("")
    lines.append(table_header)
    lines.append(table_sep)
    lines.extend(build_table(platform, platforms[platform], main_sizes, release_sizes))
    lines.append("")
    lines.append("</details>")
    lines.append("")

lines.append("<details>")
lines.append("<summary>Investigating size changes</summary>")
lines.append("")
lines.append("Use [go-size-analyzer](https://github.com/Zxilly/go-size-analyzer) to compare binaries:")
lines.append("")
lines.append("```bash")
lines.append("GOEXPERIMENT=jsonv2 go install github.com/Zxilly/go-size-analyzer/cmd/gsa@latest")
lines.append("gsa diff --old <baseline-binary> --new <new-binary>")
lines.append("```")
lines.append("")
lines.append("</details>")
lines.append("")

print("\n".join(lines))
EOF
)

if [[ -z "$COMMENT_BODY" ]]; then
     echo "No report to post."
     exit 0
fi

# Find existing comment with our marker, authored by github-actions[bot]
EXISTING_COMMENT_ID=$(gh api "repos/{owner}/{repo}/issues/${PR_NUMBER}/comments" \
     --jq ".[] | select(.user.login == \"github-actions[bot]\" and (.body | startswith(\"${MARKER}\"))) | .id" 2>/dev/null || true)
EXISTING_COMMENT_ID=$(echo "$EXISTING_COMMENT_ID" | head -1)

if [[ -n "$EXISTING_COMMENT_ID" ]]; then
     echo "Updating existing comment ${EXISTING_COMMENT_ID}..."
     gh api "repos/{owner}/{repo}/issues/comments/${EXISTING_COMMENT_ID}" \
          -X PATCH -f body="$COMMENT_BODY"
else
     echo "Posting new comment..."
     gh pr comment "$PR_NUMBER" --body "$COMMENT_BODY"
fi

echo "Done."
