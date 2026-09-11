#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# aws/setup.sh, gcp_gce platform: the exact trust policy sent to IAM, and the
# create / up-to-date / replace-stale / append-other-principal transitions.

set -u
TEST_DIR="${TEST_DIR:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}"
. "${TEST_DIR}/helpers.sh"

SA_ID="107826487864658064577"

aws_fake() {
     make_fake aws <<'EOF'
case "$*" in
*"--version"*) printf 'aws-cli/2.22.0 Python/3.12.0\n' ;;
*"sts get-caller-identity"*) printf '111122223333\tarn:aws:iam::111122223333:user/tester\n' ;;
*"iam list-account-aliases"*) printf 'None\n' ;;
*"iam get-role"*"Role.AssumeRolePolicyDocument"*)
     [ -f "${SANDBOX}/existing_trust.json" ] || exit 254
     cat "${SANDBOX}/existing_trust.json"
     ;;
*"iam get-role"*"Role.Arn"*)
     prev=""
     name=""
     for a in "$@"; do
          [ "${prev}" = "--role-name" ] && name="${a}"
          prev="${a}"
     done
     printf 'arn:aws:iam::111122223333:role/%s\n' "${name}"
     ;;
*"iam create-role"*)
     prev=""
     for a in "$@"; do
          case "${prev}" in
          --assume-role-policy-document) printf '%s' "${a}" >"${SANDBOX}/created_trust.json" ;;
          --path) printf '%s' "${a}" >"${SANDBOX}/created_path.txt" ;;
          esac
          prev="${a}"
     done
     ;;
*"iam update-assume-role-policy"*)
     prev=""
     for a in "$@"; do
          [ "${prev}" = "--policy-document" ] && printf '%s' "${a}" >"${SANDBOX}/updated_trust.json"
          prev="${a}"
     done
     ;;
*"iam attach-role-policy"*) : ;;
*"xray get-trace-segment-destination"*) printf 'CloudWatchLogs\n' ;;
*) : ;;
esac
EOF
}

run_gce_trust() {
     run_setup aws/setup.sh \
          CWAGENT_PLATFORM=gcp_gce \
          CWAGENT_AWS_REGION=us-east-1 \
          CWAGENT_GCP_SA_UNIQUE_ID="${1:-${SA_ID}}"
}

current_policy() {
     cat <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "CWAgentGCE${SA_ID}",
      "Effect": "Allow",
      "Principal": { "Federated": "accounts.google.com" },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "accounts.google.com:aud": "${SA_ID}",
          "accounts.google.com:sub": "${SA_ID}",
          "accounts.google.com:oaud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
EOF
}

t_case "fresh role: create-role receives the three-condition Google trust policy"
aws_fake
run_gce_trust
assert_status 0
assert_called "iam create-role --role-name CloudWatchAgentServerRole"
assert_called "iam attach-role-policy"
assert_output_contains stdout "Creating IAM role 'CloudWatchAgentServerRole'"
current_policy | assert_json_eq "${SANDBOX}/created_trust.json"

t_case "role with the current trust: no create, no update"
aws_fake
current_policy >"${SANDBOX}/existing_trust.json"
run_gce_trust
assert_status 0
assert_output_contains stdout "trust policy up to date"
assert_not_called "iam create-role"
assert_not_called "iam update-assume-role-policy"

t_case "stale trust for this principal: replaced, not duplicated"
aws_fake
current_policy | jq '.Statement[0].Condition.StringEquals["accounts.google.com:sub"] = "000000000000000000000"
     | .Statement[0].Condition.StringEquals["accounts.google.com:aud"] = "000000000000000000000"' \
     >"${SANDBOX}/existing_trust.json"
run_gce_trust
assert_status 0
assert_output_contains stdout "Updating trust statement"
assert_not_called "iam create-role"
assert_jq "${SANDBOX}/updated_trust.json" '.Statement | length == 1'
assert_jq "${SANDBOX}/updated_trust.json" \
     ".Statement[0].Condition.StringEquals[\"accounts.google.com:sub\"] == \"${SA_ID}\""

t_case "trust for another principal: appended, existing statement preserved"
aws_fake
cat >"${SANDBOX}/existing_trust.json" <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "Service": "ec2.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF
run_gce_trust
assert_status 0
assert_output_contains stdout "Merging trust statement"
assert_jq "${SANDBOX}/updated_trust.json" '.Statement | length == 2'
assert_jq "${SANDBOX}/updated_trust.json" '.Statement | any(.Principal.Service? == "ec2.amazonaws.com")'
assert_jq "${SANDBOX}/updated_trust.json" '.Statement | any(.Principal.Federated? == "accounts.google.com")'

t_case "second service account: appended alongside the first"
aws_fake
SA_ID_2="212121212121212121212"
current_policy >"${SANDBOX}/existing_trust.json"
run_gce_trust "${SA_ID_2}"
assert_status 0
assert_output_contains stdout "Merging trust statement"
assert_not_called "iam create-role"
assert_jq "${SANDBOX}/updated_trust.json" '.Statement | length == 2'
assert_jq "${SANDBOX}/updated_trust.json" ".Statement | any(.Sid? == \"CWAgentGCE${SA_ID}\")"
assert_jq "${SANDBOX}/updated_trust.json" ".Statement | any(.Sid? == \"CWAgentGCE${SA_ID_2}\")"
assert_jq "${SANDBOX}/updated_trust.json" \
     ".Statement | any(.Condition.StringEquals[\"accounts.google.com:sub\"]? == \"${SA_ID}\")"
assert_jq "${SANDBOX}/updated_trust.json" \
     ".Statement | any(.Condition.StringEquals[\"accounts.google.com:sub\"]? == \"${SA_ID_2}\")"

t_case "pre-Sid statement for this service account: upgraded in place"
aws_fake
current_policy | jq 'del(.Statement[0].Sid)' >"${SANDBOX}/existing_trust.json"
run_gce_trust
assert_status 0
assert_output_contains stdout "Updating trust statement"
assert_jq "${SANDBOX}/updated_trust.json" '.Statement | length == 1'
assert_jq "${SANDBOX}/updated_trust.json" ".Statement[0].Sid == \"CWAgentGCE${SA_ID}\""

t_case "pre-Sid statement for another service account: replaced by the keyed form"
aws_fake
current_policy | jq 'del(.Statement[0].Sid)
     | .Statement[0].Condition.StringEquals["accounts.google.com:sub"] = "000000000000000000000"
     | .Statement[0].Condition.StringEquals["accounts.google.com:aud"] = "000000000000000000000"' \
     >"${SANDBOX}/existing_trust.json"
run_gce_trust
assert_status 0
assert_output_contains stdout "Updating trust statement"
assert_jq "${SANDBOX}/updated_trust.json" '.Statement | length == 1'
assert_jq "${SANDBOX}/updated_trust.json" \
     ".Statement[0].Condition.StringEquals[\"accounts.google.com:sub\"] == \"${SA_ID}\""

t_case "missing CWAGENT_GCP_SA_UNIQUE_ID: dies before touching IAM"
aws_fake
run_setup aws/setup.sh CWAGENT_PLATFORM=gcp_gce CWAGENT_AWS_REGION=us-east-1
assert_status 1
assert_output_contains stderr "CWAGENT_GCP_SA_UNIQUE_ID is required for gcp_gce"
assert_not_called "iam create-role"
assert_not_called "iam update-assume-role-policy"

t_end
