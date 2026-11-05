#!/bin/bash
# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Summarize the jobs of a Test Artifacts run as test-result.json.
#
# Reads the run's jobs, one GitHub API job object per line (the output of
# `gh api --paginate .../actions/runs/<id>/jobs?filter=latest --jq '.jobs[]'`),
# and writes the result document to stdout.
#
# The verdict is "pass" when at least one job was judged and every judged job
# ended in success, skipped or neutral. A job is judged unless its test name carries the "[WIP] " prefix
# the test matrices use for work-in-progress tests; WIP jobs are listed but do
# not affect the verdict. The job running this script is excluded by name.
#
# Required env: REQUEST_ID, BUILD_ID, RUN_ID, RUN_ATTEMPT, RUN_URL, RESULT_JOB_NAME
# Optional env: TEST_REPO_BRANCH, TEST_OS_FILTER, TEST_DIR_FILTER, HELM_CHART_BRANCH
# Usage: test-result.sh <jobs.ndjson>

set -euo pipefail

JOBS_FILE="${1:?usage: test-result.sh <jobs.ndjson>}"
: "${REQUEST_ID:?REQUEST_ID is required}"
: "${BUILD_ID:?BUILD_ID is required}"
: "${RUN_ID:?RUN_ID is required}"
: "${RUN_ATTEMPT:?RUN_ATTEMPT is required}"
: "${RUN_URL:?RUN_URL is required}"
: "${RESULT_JOB_NAME:?RESULT_JOB_NAME is required}"

jq -s \
     --arg request_id "$REQUEST_ID" \
     --arg build_id "$BUILD_ID" \
     --arg run_id "$RUN_ID" \
     --arg run_attempt "$RUN_ATTEMPT" \
     --arg run_url "$RUN_URL" \
     --arg self "$RESULT_JOB_NAME" \
     --arg test_repo_branch "${TEST_REPO_BRANCH:-}" \
     --arg test_os_filter "${TEST_OS_FILTER:-}" \
     --arg test_dir_filter "${TEST_DIR_FILTER:-}" \
     --arg helm_chart_branch "${HELM_CHART_BRANCH:-}" '
  # Jobs of a called workflow are named "<caller job> / <job>"; matrix test
  # jobs are named "<os or test type>:<test>", optionally prefixed "[WIP] ".
  def parse:
    .name as $name
    | ($name | index(" / ")) as $sep
    | (if $sep then $name[0:$sep] else null end) as $group
    | (if $sep then $name[$sep + 3:] else $name end) as $rest
    | ($rest | startswith("[WIP] ")) as $wip
    | (if $wip then $rest[6:] else $rest end) as $id
    | ($id | index(":")) as $colon
    | {
        name: $name,
        group: $group,
        target: (if $colon and ($id | startswith("${{") | not) then $id[0:$colon] else null end),
        test: (if $colon and ($id | startswith("${{") | not) then $id[$colon + 1:] else $id end),
        wip: $wip,
        status: .status,
        conclusion: .conclusion,
        run_attempt: .run_attempt,
        url: .html_url,
        started_at: .started_at,
        completed_at: .completed_at
      };
  def passed: .conclusion == "success" or .conclusion == "skipped" or .conclusion == "neutral";
  def nullable: if . == "" then null else . end;

  [ .[] | select(.name != $self) | parse ] as $jobs
  | [ $jobs[] | select(.wip | not) ] as $judged
  | [ $judged[] | select(passed | not) ] as $failed
  | {
      schema_version: 1,
      request_id: $request_id,
      build_id: $build_id,
      run: { id: ($run_id | tonumber), attempt: ($run_attempt | tonumber), url: $run_url },
      plan: {
        test_repo_branch: ($test_repo_branch | nullable),
        test_os_filter: ($test_os_filter | nullable),
        test_dir_filter: ($test_dir_filter | nullable),
        helm_chart_branch: ($helm_chart_branch | nullable)
      },
      verdict: (if ($failed | length) == 0 and ($judged | length) > 0 then "pass" else "fail" end),
      summary: {
        jobs: ($jobs | length),
        judged: ($judged | length),
        wip: (($jobs | length) - ($judged | length)),
        failed: ($failed | length),
        passed_on_rerun: ([ $judged[] | select(.conclusion == "success" and .run_attempt > 1) ] | length),
        by_conclusion: ($jobs | group_by(.conclusion // "incomplete") | map({ key: (.[0].conclusion // "incomplete"), value: length }) | from_entries)
      },
      failed: [ $failed[] | .name ],
      jobs: $jobs
    }
' "$JOBS_FILE"
