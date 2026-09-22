#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# aws/setup.sh: CWAGENT_AWS_ROLE_ARN validation. Malformed ARNs, partition and
# account mismatches, and role-name conflicts fail before any IAM mutation;
# valid ARNs drive the role name and path used downstream.

set -u
TEST_DIR="${TEST_DIR:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}"
. "${TEST_DIR}/helpers.sh"

aws_fake() {
     make_fake aws <<'EOF'
case "$*" in
*"--version"*) printf 'aws-cli/2.22.0 Python/3.12.0\n' ;;
*"sts get-caller-identity"*) printf '111122223333\tarn:aws:iam::111122223333:user/tester\n' ;;
*"iam list-account-aliases"*) printf 'None\n' ;;
*"iam get-role"*"Role.AssumeRolePolicyDocument"*) exit 254 ;;
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
          [ "${prev}" = "--path" ] && printf '%s' "${a}" >"${SANDBOX}/created_path.txt"
          prev="${a}"
     done
     ;;
*"iam attach-role-policy"*) : ;;
*"xray get-trace-segment-destination"*) printf 'CloudWatchLogs\n' ;;
*) : ;;
esac
EOF
}

# run_with_arn <role arn> [extra KEY=value ...]
run_with_arn() {
     _arn="$1"
     shift
     run_setup aws/setup.sh \
          CWAGENT_PLATFORM=gcp_gce \
          CWAGENT_AWS_REGION=us-east-1 \
          CWAGENT_GCP_SA_UNIQUE_ID=107826487864658064577 \
          CWAGENT_AWS_ROLE_ARN="${_arn}" "$@"
}

expect_rejected() {
     assert_status 1
     assert_output_contains stderr "$1"
     assert_not_called "iam create-role"
     assert_not_called "iam update-assume-role-policy"
}

t_case "malformed ARN is rejected"
aws_fake
run_with_arn "not-an-arn"
expect_rejected "invalid IAM role ARN"

t_case "ARN whose account ID is not 12 digits is rejected"
aws_fake
run_with_arn "arn:aws:iam::12345678901:role/MyRole"
expect_rejected "the account ID must be 12 digits"

t_case "ARN in another partition is rejected"
aws_fake
run_with_arn "arn:aws-cn:iam::111122223333:role/MyRole"
expect_rejected "Rerun with credentials for the 'aws-cn' partition"

t_case "ARN for another account is rejected"
aws_fake
run_with_arn "arn:aws:iam::999988887777:role/MyRole"
expect_rejected "Rerun with credentials for account '999988887777'"

t_case "role name disagreeing with the ARN is rejected"
aws_fake
run_with_arn "arn:aws:iam::111122223333:role/Custom" CWAGENT_AWS_ROLE_NAME=Other
expect_rejected "conflicts with CWAGENT_AWS_ROLE_ARN"

t_case "role name agreeing with the ARN proceeds under the ARN's name"
aws_fake
run_with_arn "arn:aws:iam::111122223333:role/Custom" CWAGENT_AWS_ROLE_NAME=Custom
assert_status 0
assert_output_contains stdout "Role ARN account matches this shell's credentials"
assert_called "iam create-role --role-name Custom"

t_case "pathed ARN resolves to the last segment and carries its path into create-role"
aws_fake
run_with_arn "arn:aws:iam::111122223333:role/eng/team/Deep"
assert_status 0
assert_called "iam create-role --role-name Deep"
if [ "$(cat "${SANDBOX}/created_path.txt" 2>/dev/null)" != "/eng/team/" ]; then
     fail "create-role --path was '$(cat "${SANDBOX}/created_path.txt" 2>/dev/null)', want /eng/team/"
fi

t_end
