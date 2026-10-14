#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Regression test for https://github.com/aws/amazon-cloudwatch-agent/issues/1218
# Usage: amazon-cloudwatch-agent-ctl_test.sh <ctl-script>

set -eu

ctl="${1:?usage: amazon-cloudwatch-agent-ctl_test.sh <ctl-script>}"
export AMAZON_CLOUDWATCH_AGENT_CTL_SOURCE_ONLY=1
# shellcheck disable=SC1090
. "${ctl}"

fail() {
     printf 'FAIL: %s\n' "$1" >&2
     exit 1
}

tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT
legacy="${tmp}/amazon-cloudwatch-agent.json"

reset_legacy() {
     printf '%s\n' '{}' >"${legacy}"
}

assert_kept() {
     reset_legacy
     remove_replaced_legacy_json "$1" "${legacy}"
     if [ ! -f "${legacy}" ]; then
          fail "deleted source: $1"
     fi
}

assert_removed() {
     reset_legacy
     remove_replaced_legacy_json "$1" "${legacy}"
     if [ -f "${legacy}" ]; then
          fail "kept legacy json for: $1"
     fi
}

assert_kept "file:${legacy}"

(
     cd "${tmp}" || exit 1
     reset_legacy
     remove_replaced_legacy_json "file:./amazon-cloudwatch-agent.json" "${legacy}"
)
if [ ! -f "${legacy}" ]; then
     fail "deleted relative source"
fi

ln -s "${legacy}" "${tmp}/alias.json"
assert_kept "file:${tmp}/alias.json"
rm -f "${tmp}/alias.json"

mkdir -p "${tmp}/sub"
assert_kept "file:${tmp}/sub/../amazon-cloudwatch-agent.json"

printf '%s\n' '{}' >"${tmp}/other.json"
assert_removed "file:${tmp}/other.json"
assert_removed "ssm:AmazonCloudWatch-Config.json"
assert_removed "default"
assert_removed "file:"

printf 'ok %s\n' "${ctl}"
