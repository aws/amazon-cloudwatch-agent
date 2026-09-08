#!/bin/bash
# Collect local binary sizes and write a JSON artifact.
# The report job handles baseline comparison.
#
# Required env: RUNNER_OS

set -euo pipefail

RUNNER_OS="${RUNNER_OS:-unknown}"

echo "Collecting local binary sizes..."

python3 - "$RUNNER_OS" <<'PYEOF'
import json, os, sys

runner_os = sys.argv[1]
skip_ext = ('.sig', '.jar', '.rpm', '.deb', '.pkg', '.msi', '.tar.gz', '.gz', '.zip')
skip_names = ('CWAGENT_VERSION', 'buildMSI.zip')

binaries = {}
for root, dirs, files in os.walk("build/bin"):
    for fname in files:
        filepath = os.path.join(root, fname)
        rel_path = os.path.relpath(filepath, "build/bin")
        segments = rel_path.replace("\\", "/").split("/")
        if len(segments) != 2:
            continue
        if any(fname.endswith(ext) for ext in skip_ext):
            continue
        if fname in skip_names:
            continue
        key = "/".join(segments)
        binaries[key] = os.path.getsize(filepath)

output_file = f"size-report-{runner_os}.json"
with open(output_file, "w") as f:
    json.dump({"binaries": binaries}, f, indent=2)

print(f"Wrote {output_file} ({len(binaries)} binaries)")
PYEOF
