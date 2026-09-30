#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# gcp/setup.sh install mode (CWAGENT_AWS_ROLE_ARN set) on a GCE VM: like the
# Azure path, the remote stdout (success sentinel + status JSON) is printed and
# the remote stderr (install transcript) is shown only when the install fails.

set -u
TEST_DIR="${TEST_DIR:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}"
. "${TEST_DIR}/helpers.sh"

ROLE_ARN="arn:aws:iam::123456789012:role/CloudWatchAgentServerRole-test"

# Fake gcloud. The install ssh replays what install.sh prints: the transcript
# on stderr, the sentinel and status JSON on stdout. Sandbox flags pick the
# variant: install_fails, status_stopped, no_stdout, crlf, windows_vm,
# large_transcript.
gcloud_fake() {
     make_fake gcloud <<'EOF'
case "$*" in
*"auth list"*) printf 'tester@example.com\n' ;;
*"projects describe"*) printf 'test-project\n' ;;
*"compute instances describe"*)
     if [ -f "${SANDBOX}/windows_vm" ]; then
          printf 'test-sa@test-project.iam.gserviceaccount.com\thttps://www.googleapis.com/compute/v1/projects/windows-cloud/global/licenses/windows-server-2022-dc\n'
     else
          printf 'test-sa@test-project.iam.gserviceaccount.com\thttps://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/licenses/ubuntu-2204-lts\n'
     fi
     ;;
*"iam service-accounts describe"*) printf '107826487864658064577\n' ;;
*"compute ssh"*"--command curl"* | *"compute ssh"*"--command powershell"*)
     printf 'Installing package...\n' >&2
     printf 'E! [EC2] Fetch hostname from EC2 metadata fail: <!DOCTYPE html>\n' >&2
     # A running status on stderr must not count as success: only stdout does.
     printf '{\n  "status": "running"\n}\n' >&2
     # A large transcript (long lines, many of them) like the real one, so
     # stdout and stderr separation is exercised past pipe buffer sizes.
     if [ -f "${SANDBOX}/large_transcript" ]; then
          long=$(printf '%01500d' 0)
          i=0
          while [ "${i}" -lt 200 ]; do
               printf 'E! [EC2] noise %s %s\n' "${i}" "${long}" >&2
               i=$((i + 1))
          done
     fi
     if [ -f "${SANDBOX}/install_fails" ]; then
          printf 'Error: agent did not start. Check /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log\n' >&2
          exit 1
     fi
     [ -f "${SANDBOX}/no_stdout" ] && exit 0
     eol='\n'
     [ -f "${SANDBOX}/crlf" ] && eol='\r\n'
     status=running
     [ -f "${SANDBOX}/status_stopped" ] && status=stopped
     printf "Amazon CloudWatch Agent installed and running.${eol}"
     printf "{${eol}"
     printf "  \"status\": \"${status}\",${eol}"
     printf "  \"starttime\": \"2026-09-29T12:13:13+00:00\",${eol}"
     printf "  \"configstatus\": \"configured\",${eol}"
     printf "  \"version\": \"1.300073.2b1889\"${eol}"
     printf "}${eol}"
     ;;
*) : ;;
esac
EOF
}

run_install() {
     run_setup gcp/setup.sh \
          CWAGENT_PLATFORM=gcp_gce \
          CWAGENT_GCP_PROJECT=test-project \
          CWAGENT_GCP_LOCATION=us-east1-b \
          CWAGENT_GCP_INSTANCE_NAME=test-vm \
          CWAGENT_AWS_ROLE_ARN="${ROLE_ARN}" \
          CWAGENT_AWS_REGION=us-east-1
}

assert_output_lacks() {
     if grep -F -q -- "$2" "${SANDBOX}/$1"; then
          fail "$1 unexpectedly contains '$2' (got: $(cat "${SANDBOX}/$1"))"
     fi
}

t_case "linux success: stdout shown, transcript hidden"
gcloud_fake
run_install
assert_status 0
assert_output_contains stdout "Amazon CloudWatch Agent installed and running."
assert_output_contains stdout '"version": "1.300073.2b1889"'
assert_output_contains stdout "Agent installed on 'test-vm'"
assert_output_contains stdout "Service account unique ID (for the AWS setup): 107826487864658064577"
assert_output_lacks stdout "Installing package..."
assert_output_lacks stdout "EC2 metadata"
assert_output_lacks stderr "Installing package..."
assert_output_lacks stderr "EC2 metadata"
assert_called "compute ssh test-vm --zone us-east1-b --command curl -fsSL .*/install.sh"

t_case "linux success with a large transcript: stdout intact, transcript hidden"
gcloud_fake
: >"${SANDBOX}/large_transcript"
run_install
assert_status 0
assert_output_contains stdout "Amazon CloudWatch Agent installed and running."
assert_output_contains stdout '"status": "running",'
assert_output_contains stdout "Agent installed on 'test-vm'"
assert_output_lacks stdout "noise"
assert_output_lacks stderr "noise"

t_case "linux install fails with a large transcript: all of it on stderr"
gcloud_fake
: >"${SANDBOX}/large_transcript"
: >"${SANDBOX}/install_fails"
run_install
assert_status 1
assert_output_contains stderr "E! [EC2] noise 0 "
assert_output_contains stderr "E! [EC2] noise 199 "
assert_output_contains stderr "agent did not start"
if [ "$(grep -c 'E! \[EC2\] noise' "${SANDBOX}/stderr")" -ne 200 ]; then
     fail "expected all 200 transcript lines on stderr"
fi

t_case "linux success with CRLF output: no carriage returns printed"
gcloud_fake
: >"${SANDBOX}/crlf"
run_install
assert_status 0
assert_output_contains stdout "Agent installed on 'test-vm'"
if grep -q "$(printf '\r')" "${SANDBOX}/stdout"; then
     fail "stdout carries a carriage return"
fi

t_case "linux install fails: transcript on stderr, dies"
gcloud_fake
: >"${SANDBOX}/install_fails"
run_install
assert_status 1
assert_output_contains stderr "Installing package..."
assert_output_contains stderr "agent did not start"
assert_output_contains stderr "Install script failed on 'test-vm'"
assert_output_lacks stdout "Agent installed on"

t_case "linux exit 0 but stdout status not running: treated as a failure"
gcloud_fake
: >"${SANDBOX}/status_stopped"
run_install
assert_status 1
assert_output_contains stdout '"status": "stopped"'
assert_output_contains stderr "Installing package..."
assert_output_contains stderr "Install script failed on 'test-vm'"

t_case "linux exit 0 with no stdout (running JSON only on stderr): treated as a failure"
gcloud_fake
: >"${SANDBOX}/no_stdout"
run_install
assert_status 1
assert_output_contains stderr "Installing package..."
assert_output_contains stderr "Install script failed on 'test-vm'"

t_case "windows success: stdout shown, transcript hidden"
gcloud_fake
: >"${SANDBOX}/windows_vm"
: >"${SANDBOX}/crlf"
run_install
assert_status 0
assert_output_contains stdout "Amazon CloudWatch Agent installed and running."
assert_output_contains stdout "Agent installed on 'test-vm'"
assert_output_lacks stdout "Installing package..."
assert_called "compute ssh test-vm --zone us-east1-b --command powershell -NoProfile -NonInteractive -EncodedCommand"
# The encoded script removes any stale install.ps1, stops on a failed download,
# and reports errors as plain text on stderr.
ps_script=$(sed -n 's/.*-EncodedCommand \([A-Za-z0-9+/=]*\).*/\1/p' "${CALLS}" | head -n 1 |
     base64 -d | iconv -f UTF-16LE -t UTF-8)
case "${ps_script}" in
*"\$ErrorActionPreference='Stop'; try {"*"Remove-Item -Force -ErrorAction SilentlyContinue \$env:TEMP\\install.ps1;"*"-OutFile \$env:TEMP\\install.ps1 -ErrorAction Stop; & \$env:TEMP\\install.ps1 } catch { [Console]::Error.WriteLine("*"exit 1 }"*) ;;
*) fail "unexpected encoded install script: ${ps_script}" ;;
esac

t_case "windows install fails: transcript on stderr, manual command printed"
gcloud_fake
: >"${SANDBOX}/windows_vm"
: >"${SANDBOX}/install_fails"
run_install
assert_status 0
assert_output_contains stderr "agent did not start"
assert_output_contains stdout "remote install on 'test-vm' failed (see the install output above)"
assert_output_lacks stdout "may not be elevated"
assert_output_contains stdout "To install manually"
assert_output_contains stdout "Remove-Item -Force -ErrorAction SilentlyContinue \$env:TEMP\\install.ps1; Invoke-WebRequest"
assert_output_contains stdout "-OutFile \$env:TEMP\\install.ps1 -ErrorAction Stop; & \$env:TEMP\\install.ps1"

t_end
