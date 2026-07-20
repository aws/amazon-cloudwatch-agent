#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent - onboarding setup (dispatcher)
#
# Optional convenience wrapper over the standalone setup scripts, which document
# their own inputs and can be run directly instead. This one picks the platform
# and runs them in order:
#
#   aws_ec2 / aws_ecs / aws_eks
#       aws/setup.sh (trust + install, one AWS shell)
#   azure_vm / azure_aks
#       azure/setup.sh (identity) -> aws/setup.sh (trust) -> azure/setup.sh (install)
#
# azure/setup.sh sets up the Azure identity while the role ARN is unknown, and
# installs once it is known. aws/setup.sh sets up the AWS trust (plus install on
# the AWS-native platforms). The dispatcher supplies the ARN across the handoff,
# giving the Azure chain identity -> trust -> install.
#
# AWS-native platforms run in one AWS shell. The Azure platforms span clouds:
# identity and install need the Azure CLI, trust needs the AWS CLI. This runs
# whichever steps the current shell can, then prints a command to paste into the
# next shell, carrying the values produced so far.
#
# The setup scripts are always fetched over the network, never read from
# alongside this file, so this needs "curl" and outbound access even when run
# from a local copy.
#
# Usage:
#   ./setup.sh                          Interactive wizard (TTY)
#   CWAGENT_PLATFORM=aws_ec2 CWAGENT_AWS_INSTANCE_ID=i-123 ./setup.sh
#
# Environment variables:
#   CWAGENT_PLATFORM                      aws_ec2 | aws_ecs | aws_eks | azure_vm | azure_aks
#   CWAGENT_AWS_ROLE_NAME                 IAM role name (default: CloudWatchAgentServerRole)
#   CWAGENT_AWS_REGION                    AWS region telemetry is sent to
#   CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH When set (1/true/yes/on), enable Transaction
#                                         Search if it is off
#
#   EC2:
#   CWAGENT_AWS_INSTANCE_ID               EC2 instance ID
#   CWAGENT_AWS_UPDATE_INSTANCE_ROLE      When set (1/true/yes/on), attach the policy
#                                         to the role already on the instance
#
#   ECS:
#   CWAGENT_AWS_ECS_LAUNCH_TYPE           fargate | ec2
#
#   Azure:
#   CWAGENT_AZURE_RESOURCE_GROUP          Resource group
#   CWAGENT_AZURE_VM_NAME                 VM name (azure_vm only)
#
#   Kubernetes (EKS, AKS):
#   CWAGENT_K8S_CLUSTER_NAME              Cluster name

set -eu

PLATFORM="${CWAGENT_PLATFORM:-}"
ROLE_NAME="${CWAGENT_AWS_ROLE_NAME:-CloudWatchAgentServerRole}"
REGION="${CWAGENT_AWS_REGION:-}"
INSTANCE_ID="${CWAGENT_AWS_INSTANCE_ID:-}"
UPDATE_INSTANCE_ROLE="${CWAGENT_AWS_UPDATE_INSTANCE_ROLE:-}"
ENABLE_TXN_SEARCH="${CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH:-}"
CLUSTER_NAME="${CWAGENT_K8S_CLUSTER_NAME:-}"
RESOURCE_GROUP="${CWAGENT_AZURE_RESOURCE_GROUP:-}"
VM_NAME="${CWAGENT_AZURE_VM_NAME:-}"
ECS_LAUNCH_TYPE="${CWAGENT_AWS_ECS_LAUNCH_TYPE:-}"
# Identity/trust values that cross from one setup script to the next. On a
# resumed shell they arrive via the paste command, otherwise the runs below
# produce them.
TENANT_ID="${CWAGENT_AZURE_TENANT_ID:-}"
OIDC_ISSUER="${CWAGENT_AZURE_OIDC_ISSUER:-}"
ROLE_ARN="${CWAGENT_AWS_ROLE_ARN:-}"

# Where the setup scripts are fetched from and what the paste commands point at.
BASE_URL="https://raw.githubusercontent.com/aws/amazon-cloudwatch-agent/setup-scripts/scripts"

# =============================================================================
# Output helpers
# =============================================================================

if [ -t 1 ]; then
     die() {
          printf '  \033[31m✗\033[0m %s\n' "$1" >&2
          exit 1
     }
     ask() { printf '\033[1m▸ %s\033[0m ' "$1"; }
else
     die() {
          printf '  ✗ %s\n' "$1" >&2
          exit 1
     }
     ask() { printf '▸ %s ' "$1"; }
fi

usage() {
     rc="${1:-1}"
     out=2
     [ "${rc}" = "0" ] && out=1
     cat >&${out} <<EOF
Usage:
  $0                    Interactive wizard (TTY)

  Or via environment variables:
  CWAGENT_PLATFORM=aws_ec2 CWAGENT_AWS_INSTANCE_ID=i-123 $0

Environment variables:
  CWAGENT_PLATFORM                        aws_ec2 | aws_ecs | aws_eks | azure_vm | azure_aks
  CWAGENT_AWS_ROLE_NAME                   IAM role name (default: CloudWatchAgentServerRole)
  CWAGENT_AWS_REGION                      AWS region telemetry is sent to
  CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH   Enable Transaction Search in the region

  EC2:
  CWAGENT_AWS_INSTANCE_ID                 EC2 instance ID
  CWAGENT_AWS_UPDATE_INSTANCE_ROLE        Attach the policy to the existing role

  ECS:
  CWAGENT_AWS_ECS_LAUNCH_TYPE             fargate | ec2

  Azure:
  CWAGENT_AZURE_RESOURCE_GROUP            Resource group
  CWAGENT_AZURE_VM_NAME                   VM name (azure_vm only)

  Kubernetes (EKS, AKS):
  CWAGENT_K8S_CLUSTER_NAME                Cluster name
EOF
     exit "${rc}"
}

# =============================================================================
# Interactive wizard
#
# Collects the platform and its inputs up front so the setup scripts can run
# non-interactively (they are invoked with the values already in the
# environment, and under CWAGENT_EMIT_ENV they cannot prompt at all).
# =============================================================================

prompt() {
     var="$1"
     label="$2"
     default="${3:-}"
     eval "current=\${${var}}"
     [ -n "${current}" ] && return
     while true; do
          if [ -n "${default}" ]; then
               ask "${label} [${default}]:"
          else
               ask "${label}:"
          fi
          # A read failure (EOF on closed stdin) would loop forever with no
          # default, and abort under set -e otherwise, so treat it as give-up.
          read -r input || die "no input for ${label}"
          input="${input:-${default}}"
          [ -n "${input}" ] && break
     done
     # Assign without re-quoting the value, so a pasted name containing " ` or
     # $(...) is stored literally rather than being re-parsed by the shell.
     eval "${var}=\$input"
}

interactive_setup() {
     if [ -z "${PLATFORM}" ]; then
          printf '\nSelect platform:\n'
          printf '  aws_ec2     EC2 instance\n'
          printf '  aws_ecs     ECS task (sidecar)\n'
          printf '  aws_eks     EKS cluster (add-on)\n'
          printf '  azure_vm    Azure VM\n'
          printf '  azure_aks   AKS cluster (Helm)\n'
          ask "Platform:"
          read -r PLATFORM || die "no platform selected"
     fi

     printf '\n'
     case "${PLATFORM}" in
     aws_ec2) prompt INSTANCE_ID "Instance ID" ;;
     aws_ecs) prompt ECS_LAUNCH_TYPE "Launch type (fargate|ec2)" "fargate" ;;
     aws_eks)
          prompt CLUSTER_NAME "Cluster name"
          ;;
     azure_vm)
          prompt RESOURCE_GROUP "Resource group"
          prompt VM_NAME "VM name"
          ;;
     azure_aks)
          prompt RESOURCE_GROUP "Resource group"
          prompt CLUSTER_NAME "Cluster name"
          ;;
     *) die "invalid platform: ${PLATFORM}" ;;
     esac

     # Offer the AWS CLI's configured region as the default when present.
     if [ -z "${REGION}" ] && have_aws; then
          REGION=$(aws configure get region 2>/dev/null || true)
     fi
     prompt REGION "AWS region telemetry is sent to" "${REGION}"
     prompt ROLE_NAME "IAM role name" "${ROLE_NAME}"
}

# =============================================================================
# Script runners
# =============================================================================

have_az() { command -v az >/dev/null 2>&1; }
have_aws() { command -v aws >/dev/null 2>&1; }

# Export everything the setup scripts read, so a plain "sh <script>" inherits it.
# Empty values are harmless: the scripts fall back to their own defaults.
export_env() {
     export CWAGENT_PLATFORM="${PLATFORM}"
     export CWAGENT_AWS_ROLE_NAME="${ROLE_NAME}"
     export CWAGENT_AWS_REGION="${REGION}"
     export CWAGENT_AWS_INSTANCE_ID="${INSTANCE_ID}"
     export CWAGENT_AWS_UPDATE_INSTANCE_ROLE="${UPDATE_INSTANCE_ROLE}"
     export CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH="${ENABLE_TXN_SEARCH}"
     export CWAGENT_K8S_CLUSTER_NAME="${CLUSTER_NAME}"
     export CWAGENT_AZURE_RESOURCE_GROUP="${RESOURCE_GROUP}"
     export CWAGENT_AZURE_VM_NAME="${VM_NAME}"
     export CWAGENT_AWS_ECS_LAUNCH_TYPE="${ECS_LAUNCH_TYPE}"
     export CWAGENT_AZURE_TENANT_ID="${TENANT_ID}"
     export CWAGENT_AZURE_OIDC_ISSUER="${OIDC_ISSUER}"
     export CWAGENT_AWS_ROLE_ARN="${ROLE_ARN}"
}

# Fetch a hosted setup script to stdout, failing loudly if it cannot be had.
# Piping curl straight into sh would hide the download error: the pipeline takes
# sh's status, and sh reading empty stdin exits 0, so a 404 looks like a script
# that ran and did nothing.
fetch_script() {
     rel="$1"
     command -v curl >/dev/null 2>&1 || die "curl is required to fetch ${rel}"
     curl -fsSL "${BASE_URL}/${rel}" || die "could not fetch ${BASE_URL}/${rel}"
}

# Run a setup script for its readable output. CWAGENT_EMIT_ENV is unset so its
# progress prints normally. Fetch into a variable first (see fetch_script): a
# piped fetch_script would mask a 404.
run_script() {
     rel="$1"
     export_env
     script=$(fetch_script "${rel}")
     printf '%s' "${script}" | CWAGENT_EMIT_ENV='' sh
}

# The identity/trust values a script may hand back, each folded into a local below.
EMITTED_KEYS="CWAGENT_AZURE_TENANT_ID
CWAGENT_AZURE_OIDC_ISSUER
CWAGENT_AWS_ROLE_ARN
CWAGENT_AWS_REGION
CWAGENT_K8S_CLUSTER_NAME"

# Run a script under CWAGENT_EMIT_ENV and fold its emitted CWAGENT_* values back
# into this shell, so the next run and any paste command pick them up. The
# script routes its own logging to stderr, so progress is still visible.
load_script() {
     rel="$1"
     export_env
     # Fetch first (see run_script): a piped fetch_script would mask a 404.
     script=$(fetch_script "${rel}")
     emitted=$(printf '%s' "${script}" | CWAGENT_EMIT_ENV=1 sh)
     # A script's stdout is untrusted, so eval only lines matching an allowlisted
     # key in the exact KEY='value' form. An unfiltered eval would let a stray
     # line redefine PATH or a download URL.
     for key in ${EMITTED_KEYS}; do
          safe_line=$(printf '%s\n' "${emitted}" | grep -E "^${key}='[^']*'\$" | tail -1 || true)
          [ -n "${safe_line}" ] || continue
          eval "export ${safe_line}"
     done
     TENANT_ID="${CWAGENT_AZURE_TENANT_ID:-${TENANT_ID}}"
     OIDC_ISSUER="${CWAGENT_AZURE_OIDC_ISSUER:-${OIDC_ISSUER}}"
     ROLE_ARN="${CWAGENT_AWS_ROLE_ARN:-${ROLE_ARN}}"
     CLUSTER_NAME="${CWAGENT_K8S_CLUSTER_NAME:-${CLUSTER_NAME}}"
     REGION="${CWAGENT_AWS_REGION:-${REGION}}"
}

# Assert a run produced the value the next one needs. A run that failed to
# fetch or died early still lets the chain continue otherwise, and the symptom
# surfaces later as a resume command silently missing a key.
require_output() {
     [ -n "$2" ] || die "$1 did not produce $3. Rerun it."
}

# The identity run yields the tenant ID for a VM and the OIDC issuer for AKS.
require_identity_output() {
     case "${PLATFORM}" in
     azure_vm) require_output "azure/setup.sh" "${TENANT_ID}" "a tenant ID" ;;
     azure_aks) require_output "azure/setup.sh" "${OIDC_ISSUER}" "an OIDC issuer URL" ;;
     esac
}

# Print the command to re-run this script in the next shell. Over-includes the
# known values: this script re-derives which step to run from which are set, so
# a superset is harmless and keeps the paste one self-contained block.
print_resume() {
     hint="$1"
     block=""
     add_kv() {
          [ -n "$2" ] || return 0
          block="${block}      $1='$2' \\
"
     }
     add_kv CWAGENT_PLATFORM "${PLATFORM}"
     add_kv CWAGENT_AZURE_TENANT_ID "${TENANT_ID}"
     add_kv CWAGENT_AZURE_OIDC_ISSUER "${OIDC_ISSUER}"
     add_kv CWAGENT_AWS_ROLE_ARN "${ROLE_ARN}"
     add_kv CWAGENT_AZURE_RESOURCE_GROUP "${RESOURCE_GROUP}"
     add_kv CWAGENT_AZURE_VM_NAME "${VM_NAME}"
     add_kv CWAGENT_K8S_CLUSTER_NAME "${CLUSTER_NAME}"
     add_kv CWAGENT_AWS_REGION "${REGION}"
     [ "${ROLE_NAME}" = "CloudWatchAgentServerRole" ] || add_kv CWAGENT_AWS_ROLE_NAME "${ROLE_NAME}"

     printf '\nNext: run this in %s\n\n' "${hint}"
     # Strip the first line's indent so the pipe aligns with it: the paste reads
     # "curl ... \ | KEY='v' \ <indented KEYs> \ sh".
     printf '  curl -fsSL %s/setup.sh \\\n    | %s      sh\n' "${BASE_URL}" "${block#      }"
}

# =============================================================================
# Orchestration
# =============================================================================

# AWS-native platforms run entirely in one AWS shell.
orchestrate_aws() {
     have_aws || die "AWS CLI is required for ${PLATFORM} (run in a shell that has it, e.g. AWS CloudShell)"
     run_script aws/setup.sh
}

# Azure platforms span clouds. identity_done / trust_done are read off the
# values already in the environment, so each shell resumes at the right step.
orchestrate_azure() {
     case "${PLATFORM}" in
     azure_vm) [ -n "${TENANT_ID}" ] && identity_done=1 || identity_done="" ;;
     azure_aks) [ -n "${OIDC_ISSUER}" ] && identity_done=1 || identity_done="" ;;
     esac
     [ -n "${ROLE_ARN}" ] && trust_done=1 || trust_done=""

     # Drive the ARN-unknown flow: identity first (ROLE_ARN empty), then install
     # after trust supplies the ARN.
     if have_az && have_aws; then
          # One shell with both CLIs: run the whole chain.
          if [ -z "${identity_done}" ]; then
               load_script azure/setup.sh
               require_identity_output
          fi
          if [ -z "${trust_done}" ]; then
               load_script aws/setup.sh
               require_output "aws/setup.sh" "${ROLE_ARN}" "an IAM role ARN"
          fi
          run_script azure/setup.sh
     elif have_aws; then
          # AWS shell: only the trust step belongs here.
          [ -n "${identity_done}" ] || die "run azure/setup.sh in Azure Cloud Shell first (it produces the tenant ID / OIDC issuer the trust step needs)"
          if [ -z "${trust_done}" ]; then
               load_script aws/setup.sh
               require_output "aws/setup.sh" "${ROLE_ARN}" "an IAM role ARN"
          fi
          print_resume "Azure Cloud Shell (has the Azure CLI)"
     elif have_az; then
          # Azure shell: run identity if pending, install once trust is done.
          if [ -z "${identity_done}" ]; then
               load_script azure/setup.sh
               require_identity_output
               print_resume "AWS CloudShell (has the AWS CLI)"
          elif [ -n "${trust_done}" ]; then
               run_script azure/setup.sh
          else
               print_resume "AWS CloudShell (has the AWS CLI)"
          fi
     else
          die "need the Azure CLI (identity/install) and/or the AWS CLI (trust) for ${PLATFORM}"
     fi
}

main() {
     case "${1:-}" in -h | --help) usage 0 ;; esac

     if [ -t 0 ] && [ -z "${PLATFORM}" ]; then
          interactive_setup
     elif [ -z "${PLATFORM}" ]; then
          usage
     fi

     case "${PLATFORM}" in
     aws_ec2 | aws_ecs | aws_eks) orchestrate_aws ;;
     azure_vm | azure_aks) orchestrate_azure ;;
     *) die "unsupported platform: ${PLATFORM} (valid: aws_ec2, aws_ecs, aws_eks, azure_vm, azure_aks)" ;;
     esac
}

main "$@"
