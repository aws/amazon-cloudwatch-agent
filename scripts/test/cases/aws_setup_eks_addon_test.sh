#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# aws/setup.sh, aws_eks: the Observability add-on is created or updated with
# the pipeline config (OTel Container Insights + logs on, v1 Container Insights
# and FluentBit off). Which agents config is sent depends on the add-on version
# that will run: the installed version on update, the cluster's default
# compatible version on create. From the first add-on version with a default-on
# OTLP endpoint, no agents list is sent; below it, or when the version cannot be
# read, the node agent keeps default:otel. The script never changes the add-on
# version (no --addon-version).

set -u
TEST_DIR="${TEST_DIR:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}"
. "${TEST_DIR}/helpers.sh"

# Sandbox knobs read by the fake:
#   installed_version  add-on exists, describe-addon prints this version
#   default_version    describe-addon-versions prints this default version
#   cluster_fail       describe-cluster fails
# The --configuration-values argument of create/update-addon is saved to
# ${SANDBOX}/addon_config.json.
aws_fake() {
     make_fake aws <<'EOF2'
save_config() {
     prev=""
     for a in "$@"; do
          [ "${prev}" = "--configuration-values" ] && printf '%s' "${a}" >"${SANDBOX}/addon_config.json"
          prev="${a}"
     done
}
case "$*" in
*"--version"*) printf 'aws-cli/2.22.0 Python/3.12.0\n' ;;
*"sts get-caller-identity"*) printf '111122223333\tarn:aws:iam::111122223333:user/tester\n' ;;
*"iam list-account-aliases"*) printf 'None\n' ;;
*"iam get-role"*"Role.AssumeRolePolicyDocument"*) exit 254 ;;
*"iam get-role"*"Role.Arn"*) printf 'arn:aws:iam::111122223333:role/CloudWatchAgentRole\n' ;;
*"eks describe-addon "*"amazon-cloudwatch-observability"*)
     [ -e "${SANDBOX}/installed_version" ] || exit 254
     cat "${SANDBOX}/installed_version"
     ;;
*"eks describe-cluster"*)
     [ -e "${SANDBOX}/cluster_fail" ] && exit 254
     printf '1.33\n'
     ;;
*"eks describe-addon-versions"*"--kubernetes-version 1.33"*)
     [ -e "${SANDBOX}/default_version" ] || exit 254
     cat "${SANDBOX}/default_version"
     ;;
*"eks list-pod-identity-associations"*) printf 'None\n' ;;
*"eks create-addon"*"amazon-cloudwatch-observability"*) save_config "$@" ;;
*"eks update-addon"*) save_config "$@" ;;
*"xray get-trace-segment-destination"*) printf 'CloudWatchLogs\n' ;;
*) : ;;
esac
EOF2
}

run_eks() {
     run_setup aws/setup.sh \
          CWAGENT_PLATFORM=aws_eks \
          CWAGENT_AWS_REGION=us-east-1 \
          CWAGENT_K8S_CLUSTER_NAME=test-cluster
}

expect_otlp_config() {
     assert_json_eq "${SANDBOX}/addon_config.json" <<'JSON'
{
  "containerInsights": { "enabled": false },
  "containerLogs": { "enabled": false },
  "otelContainerInsights": { "enabled": true, "logs": { "enabled": true } }
}
JSON
     assert_not_called "default:otel"
     assert_not_called "--addon-version"
}

expect_legacy_config() {
     assert_json_eq "${SANDBOX}/addon_config.json" <<'JSON'
{
  "containerInsights": { "enabled": false },
  "containerLogs": { "enabled": false },
  "otelContainerInsights": { "enabled": true, "logs": { "enabled": true } },
  "agents": [
    { "name": "cloudwatch-agent", "config": "default:otel" },
    { "name": "cloudwatch-agent-cluster-scraper", "mode": "deployment", "config": "default" }
  ]
}
JSON
     assert_not_called "--addon-version"
}

expect_create() {
     assert_status 0
     assert_called "eks create-addon --cluster-name test-cluster --addon-name amazon-cloudwatch-observability"
     assert_not_called "eks update-addon"
}

expect_update() {
     assert_status 0
     assert_called "eks update-addon --cluster-name test-cluster --addon-name amazon-cloudwatch-observability"
     assert_not_called "eks create-addon --cluster-name test-cluster --addon-name amazon-cloudwatch-observability"
}

t_case "create, default version is the OTLP version: config without default:otel"
aws_fake
printf 'v8.0.0-eksbuild.1\n' >"${SANDBOX}/default_version"
run_eks
expect_create
assert_called "eks describe-addon-versions --addon-name amazon-cloudwatch-observability --kubernetes-version 1.33"
expect_otlp_config
assert_output_contains stdout "Add-on v8.0.0-eksbuild.1 (default for this cluster) serves OTLP by default"

t_case "create, default version newer than the OTLP version: config without default:otel"
aws_fake
printf 'v8.10.2-eksbuild.3\n' >"${SANDBOX}/default_version"
run_eks
expect_create
expect_otlp_config

t_case "create, default version older: config keeps default:otel"
aws_fake
printf 'v7.0.1-eksbuild.1\n' >"${SANDBOX}/default_version"
run_eks
expect_create
expect_legacy_config
assert_output_contains stdout "Add-on v7.0.1-eksbuild.1 (default for this cluster) is older than v8.0.0-eksbuild.1"

t_case "create, cluster version lookup fails: config keeps default:otel"
aws_fake
printf 'v8.0.0-eksbuild.1\n' >"${SANDBOX}/default_version"
: >"${SANDBOX}/cluster_fail"
run_eks
expect_create
expect_legacy_config
assert_output_contains stdout "Add-on version unknown"

t_case "create, default version lookup fails: config keeps default:otel"
aws_fake
run_eks
expect_create
expect_legacy_config
assert_output_contains stdout "Add-on version unknown"

t_case "create, no default version listed: config keeps default:otel"
aws_fake
printf 'None\n' >"${SANDBOX}/default_version"
run_eks
expect_create
expect_legacy_config

t_case "update, installed version is the OTLP version: config without default:otel"
aws_fake
printf 'v8.0.0-eksbuild.1\n' >"${SANDBOX}/installed_version"
run_eks
expect_update
expect_otlp_config
assert_not_called "describe-addon-versions"
assert_output_contains stdout "Add-on v8.0.0-eksbuild.1 (installed) serves OTLP by default"

t_case "update, installed version older: config keeps default:otel, version unchanged"
aws_fake
printf 'v7.0.1-eksbuild.1\n' >"${SANDBOX}/installed_version"
run_eks
expect_update
expect_legacy_config
assert_output_contains stdout "Add-on v7.0.1-eksbuild.1 (installed) is older than v8.0.0-eksbuild.1"

t_case "update, installed version unreadable: config keeps default:otel"
aws_fake
printf 'None\n' >"${SANDBOX}/installed_version"
run_eks
expect_update
expect_legacy_config

t_end
