#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Unit-test helpers for the setup scripts. A test runs a script with fake
# cloud CLIs first on PATH and a minimal environment, then asserts on the
# exit status, the captured stdout/stderr, and the calls the fakes recorded.
#
# A case file sets TEST_DIR, sources this file, and then for each test:
#
#      t_case "description"
#      make_fake aws <<'EOF' ... EOF
#      run_setup aws/setup.sh CWAGENT_PLATFORM=... CWAGENT_AWS_REGION=...
#      assert_status 0
#
# and finishes with t_end, whose status is the file's verdict.

: "${TEST_DIR:?case files must set TEST_DIR before sourcing helpers.sh}"

REPO_ROOT=$(CDPATH='' cd -- "${TEST_DIR}/../.." && pwd)

# The scripts need real jq and the usual POSIX utilities. run_setup builds a
# minimal PATH from these plus the fakes, so no other host command leaks in.
if ! command -v jq >/dev/null 2>&1; then
     echo "jq is required to run these tests" >&2
     exit 1
fi
REAL_TOOL_PATH="$(dirname -- "$(command -v jq)"):/usr/bin:/bin"

WORK=$(mktemp -d "${TMPDIR:-/tmp}/cwa-scripts-test.XXXXXX")
trap 'rm -rf "${WORK}"' EXIT

TESTS=0
FAILURES=0
CURRENT=""

# Starts a test with a fresh sandbox: a bin/ for fakes, an empty call log, and
# a scratch area fakes and assertions share (exported as SANDBOX).
t_case() {
     CURRENT="$1"
     TESTS=$((TESTS + 1))
     SANDBOX="${WORK}/case-${TESTS}"
     BIN="${SANDBOX}/bin"
     CALLS="${SANDBOX}/calls.log"
     mkdir -p "${BIN}"
     : >"${CALLS}"
}

fail() {
     printf 'not ok: %s: %s\n' "${CURRENT}" "$1" >&2
     FAILURES=$((FAILURES + 1))
}

t_end() {
     printf '%s: %d tests, %d failed assertions\n' "${0##*/}" "${TESTS}" "${FAILURES}"
     [ "${FAILURES}" -eq 0 ]
}

# make_fake <name>, body on stdin. Every invocation is logged to ${CALLS};
# the body runs with the real arguments and sees SANDBOX and CALLS.
make_fake() {
     {
          sed "s/@NAME@/$1/" <<'PRELUDE'
#!/bin/sh
printf '%s\n' "@NAME@ $*" >>"${CALLS}"
PRELUDE
          cat
     } >"${BIN}/$1"
     chmod +x "${BIN}/$1"
}

# run_setup <script path under scripts/> [KEY=value ...]
# Runs the script through env -i so only the given variables (plus the fake
# PATH and the sandbox handles) form its environment. Never fails the caller:
# the exit status lands in STATUS, output in ${SANDBOX}/stdout and stderr.
run_setup() {
     _script="${REPO_ROOT}/scripts/$1"
     shift
     STATUS=0
     env -i PATH="${BIN}:${REAL_TOOL_PATH}" CALLS="${CALLS}" SANDBOX="${SANDBOX}" "$@" \
          sh "${_script}" >"${SANDBOX}/stdout" 2>"${SANDBOX}/stderr" || STATUS=$?
}

assert_status() {
     [ "${STATUS}" -eq "$1" ] ||
          fail "exit status ${STATUS}, want $1 (stderr: $(cat "${SANDBOX}/stderr"))"
}

# Byte-exact match of the captured stdout against stdin. This is the
# CWAGENT_EMIT_ENV contract: under it, stdout must carry the eval-able
# KEY='value' lines and nothing else.
assert_stdout_exact() {
     if ! diff -u - "${SANDBOX}/stdout" >"${SANDBOX}/stdout.diff" 2>&1; then
          fail "stdout differs from expected:
$(cat "${SANDBOX}/stdout.diff")"
     fi
}

# assert_output_contains <stdout|stderr> <fixed string>
assert_output_contains() {
     grep -F -q -- "$2" "${SANDBOX}/$1" ||
          fail "$1 does not contain '$2' (got: $(cat "${SANDBOX}/$1"))"
}

assert_called() {
     grep -q -- "$1" "${CALLS}" || fail "expected a call matching '$1'"
}

assert_not_called() {
     if grep -q -- "$1" "${CALLS}"; then
          fail "unexpected call matching '$1'"
     fi
}

# assert_json_eq <actual file>, expected JSON on stdin. Key order and
# whitespace are normalized; values must match exactly.
assert_json_eq() {
     jq -S . >"${SANDBOX}/expected.json"
     if ! jq -S . "$1" >"${SANDBOX}/actual.json" 2>/dev/null; then
          fail "$1 is missing or not valid JSON"
          return
     fi
     if ! diff -u "${SANDBOX}/expected.json" "${SANDBOX}/actual.json" >"${SANDBOX}/json.diff"; then
          fail "JSON differs:
$(cat "${SANDBOX}/json.diff")"
     fi
}

# assert_jq <file> <jq boolean expression>
assert_jq() {
     jq -e "$2" "$1" >/dev/null 2>&1 || fail "jq assertion failed on $1: $2"
}
