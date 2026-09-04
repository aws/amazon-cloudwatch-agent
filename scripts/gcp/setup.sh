#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent - GCP setup (identity + install)
#
# Discovers the GCP-side identity the agent uses and installs it:
#
#   gcp_vm      reads the service account attached to the GCE VM and its
#               unique ID (the value the AWS trust policy conditions on),
#               then pushes install.sh to it via "gcloud compute ssh",
#               printing the command instead when SSH cannot reach the VM
#               (requires: gcloud)
#   gcp_gke     reads the cluster and emits its OIDC issuer URL (the value the
#               AWS trust policy is built on), then installs the CloudWatch
#               Observability Helm chart when helm and kubectl are present,
#               otherwise prints the commands (requires: gcloud)
#
# Two modes, keyed on CWAGENT_AWS_ROLE_ARN:
#
#   Set: identity AND install in one run. Install can precede the AWS trust
#     setup and the agent retries with backoff until the trust is in place.
#
#   Unset: identity only, then stop (for anyone without the ARN yet). Run
#     aws/setup.sh to get the ARN, then rerun with it set. Identity is
#     read-only, so the rerun just installs.
#
# Usage:
#   Interactive (TTY):
#     ./gcp/setup.sh
#
#   Environment variables (piped/automated):
#     CWAGENT_PLATFORM=gcp_vm \
#     CWAGENT_AWS_ROLE_ARN=arn:aws:iam::... \
#     CWAGENT_AWS_REGION=us-east-1 \
#     CWAGENT_GCP_ZONE=us-east1-b \
#     CWAGENT_GCP_VM_NAME=my-vm \
#       ./gcp/setup.sh
#
# Environment variables:
#   Common:
#     CWAGENT_PLATFORM              gcp_vm | gcp_gke
#     CWAGENT_AWS_ROLE_ARN          IAM role ARN, install runs too when set
#     CWAGENT_AWS_REGION            AWS region telemetry is sent to (required
#                                   for install)
#     CWAGENT_GCP_PROJECT           Project ID the resource lives in (the
#                                   gcloud config default is used when unset)
#     CWAGENT_EMIT_ENV              When set (1/true/yes/on), print eval-able KEY='value'
#                                   lines on stdout and route all logging to stderr
#   gcp_vm:
#     CWAGENT_GCP_ZONE              Zone the VM lives in
#     CWAGENT_GCP_VM_NAME           VM name
#   gcp_gke:
#     CWAGENT_GCP_LOCATION          Zone or region the cluster lives in
#     CWAGENT_K8S_CLUSTER_NAME      Cluster name

set -eu

PLATFORM="${CWAGENT_PLATFORM:-}"
ROLE_ARN="${CWAGENT_AWS_ROLE_ARN:-}"
REGION="${CWAGENT_AWS_REGION:-}"
PROJECT="${CWAGENT_GCP_PROJECT:-}"
ZONE="${CWAGENT_GCP_ZONE:-}"
VM_NAME="${CWAGENT_GCP_VM_NAME:-}"
LOCATION="${CWAGENT_GCP_LOCATION:-}"
CLUSTER_NAME="${CWAGENT_K8S_CLUSTER_NAME:-}"
EMIT_ENV="${CWAGENT_EMIT_ENV:-}"
HELM_CHART_REPO="https://aws-observability.github.io/helm-charts"

# Where the VM fetches the install payload (install.sh) from.
SCRIPT_BASE_URL="https://raw.githubusercontent.com/aws/amazon-cloudwatch-agent/main/scripts"
K8S_NAMESPACE="amazon-cloudwatch"

# True when a flag is an affirmative value (1, true, yes, on, case-insensitive).
# One truthiness rule for every CWAGENT_* boolean.
is_true() {
     case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
     1 | true | yes | on) return 0 ;;
     *) return 1 ;;
     esac
}

# =============================================================================
# Output helpers
#
# Under CWAGENT_EMIT_ENV stdout carries only the eval-able KEY='value' lines the
# parent shell captures and evaluates, so readable output (including the install
# transcript) goes to fd 3 (redirected to stderr here, stdout otherwise) and
# every command must keep its own output off plain stdout.
# =============================================================================

if is_true "${EMIT_ENV}"; then
     exec 3>&2
else
     exec 3>&1
fi

section() { printf '\n%s\n' "$1" >&3; }
if [ -t 3 ]; then
     log() { printf '  \033[32m✓\033[0m %s\n' "$1" >&3; }
     logaction() { printf '  \033[33m+\033[0m %s\n' "$1" >&3; }
     logwarn() { printf '  \033[33m!\033[0m %s\n' "$1" >&3; }
     die() {
          printf '  \033[31m✗\033[0m %s\n' "$1" >&2
          exit 1
     }
     ask() { printf '\033[1m▸ %s\033[0m ' "$1" >&3; }
else
     log() { printf '  ✓ %s\n' "$1" >&3; }
     logaction() { printf '  + %s\n' "$1" >&3; }
     logwarn() { printf '  ! %s\n' "$1" >&3; }
     die() {
          printf '  ✗ %s\n' "$1" >&2
          exit 1
     }
     ask() { printf '▸ %s ' "$1" >&3; }
fi

usage() {
     rc="${1:-1}"
     out=2
     [ "${rc}" = "0" ] && out=1
     cat >&${out} <<EOF
Usage:
  $0                    Interactive wizard (TTY)

  Or via environment variables:
  CWAGENT_PLATFORM=gcp_vm CWAGENT_GCP_ZONE=us-east1-b CWAGENT_GCP_VM_NAME=my-vm $0
  CWAGENT_PLATFORM=gcp_gke CWAGENT_GCP_LOCATION=us-east1-b CWAGENT_K8S_CLUSTER_NAME=my-cluster $0

Environment variables:
  Common:
    CWAGENT_PLATFORM              gcp_vm | gcp_gke
    CWAGENT_AWS_ROLE_ARN          IAM role ARN; when set, install runs too
    CWAGENT_AWS_REGION            AWS region (required for install)
    CWAGENT_GCP_PROJECT           Project ID of the resource (the gcloud config
                                  default is used when unset)
    CWAGENT_EMIT_ENV              Print eval-able KEY='value' lines on stdout
  gcp_vm:
    CWAGENT_GCP_ZONE              Zone the VM lives in
    CWAGENT_GCP_VM_NAME           VM name
  gcp_gke:
    CWAGENT_GCP_LOCATION          Zone or region the cluster lives in
    CWAGENT_K8S_CLUSTER_NAME      Cluster name
EOF
     exit "${rc}"
}

# =============================================================================
# Env-var emit (CWAGENT_EMIT_ENV)
#
# Values are single-quoted for the documented
# eval "$(... | CWAGENT_EMIT_ENV=1 sh)" usage.
# =============================================================================

ENV_VARS=""
add_env() {
     [ -n "$2" ] || return 0
     ENV_VARS="${ENV_VARS}$1=$2
"
}

emit_env() {
     is_true "${EMIT_ENV}" || return 0
     printf '%s' "${ENV_VARS}" | while IFS= read -r line; do
          [ -n "${line}" ] || continue
          value="${line#*=}"
          # Escape single quotes (' -> '\'') so the emitted line survives the
          # documented eval intact even if a value ever contains one.
          escaped=$(printf '%s' "${value}" | sed "s/'/'\\\\''/g")
          printf "%s='%s'\n" "${line%%=*}" "${escaped}"
     done
}

# ARN-unknown mode: identity is done but install waits on the AWS trust setup.
# Emitted to fd 3 so it never lands on stdout's CWAGENT_EMIT_ENV KEY='value' lines.
print_await_arn() {
     printf '\n' >&3
     log "GCP identity discovered (install pending the IAM role ARN)"
     printf '\nNext: run aws/setup.sh with the identity value above to create the IAM\n' >&3
     printf 'role and trust, then rerun this with CWAGENT_AWS_ROLE_ARN set to install the\n' >&3
     printf 'agent.\n' >&3
}

# =============================================================================
# Interactive mode
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
     printf '\nSelect platform:\n' >&3
     printf '  gcp_vm      GCE VM\n' >&3
     printf '  gcp_gke     GKE cluster\n' >&3
     ask "Platform:"
     read -r choice || die "no platform selected"
     case "${choice}" in
     gcp_vm) PLATFORM=gcp_vm ;;
     gcp_gke) PLATFORM=gcp_gke ;;
     *) die "invalid platform: ${choice}" ;;
     esac

     printf '\n' >&3
     case "${PLATFORM}" in
     gcp_vm)
          prompt ZONE "Zone"
          prompt VM_NAME "VM name"
          ;;
     gcp_gke)
          prompt LOCATION "Location (zone or region)"
          prompt CLUSTER_NAME "Cluster name"
          ;;
     esac
}

# Pin gcloud to the target project without gcloud config set mutating the
# user's persistent default. PROJECT is always resolved by the time this runs
# (check_prerequisites fills it from the config default when unset).
gcloud_scoped() {
     gcloud "$@" --project "${PROJECT}"
}

check_prerequisites() {
     command -v gcloud >/dev/null 2>&1 || die "Google Cloud CLI is required but not installed"
     ACTIVE_ACCOUNT=$(gcloud auth list --filter=status:ACTIVE --format='value(account)' 2>/dev/null | head -n 1)
     [ -n "${ACTIVE_ACCOUNT}" ] || die "not logged in to gcloud (run 'gcloud auth login')"
     # Project comes from the environment or the gcloud config default, like
     # the Azure script's ambient subscription. Resolved here so later steps
     # (and the emitted env) carry a concrete value.
     if [ -z "${PROJECT}" ]; then
          PROJECT=$(gcloud config get-value project 2>/dev/null || true)
          [ "${PROJECT}" = "(unset)" ] && PROJECT=""
     fi
     [ -n "${PROJECT}" ] || die "no GCP project (set CWAGENT_GCP_PROJECT or run 'gcloud config set project <project-id>')"
     # Describe doubles as the existence and access check, like the Azure
     # script's scoped account show.
     gcloud projects describe "${PROJECT}" --format='value(projectId)' >/dev/null 2>&1 ||
          die "cannot access project ${PROJECT} (run 'gcloud auth login'; check 'gcloud projects list')"
     log "GCP account: ${ACTIVE_ACCOUNT}"
     log "GCP project: ${PROJECT}"
}

# =============================================================================
# Install payload builders (VM path)
# =============================================================================

# Emit the one-line command that fetches install.sh and runs it on the VM,
# with the given env assignments as a prefix. Carries its own sudo: unlike
# Azure's run-command, gcloud compute ssh runs as the SSH user, not root.
# $1 = env assignments.
linux_install_cmd() {
     envs="$1"
     printf 'curl -fsSL %s/install.sh | sudo %s sh' "${SCRIPT_BASE_URL}" "${envs}"
}

# Run a command on the GCE VM via gcloud compute ssh. $1 = command.
# ssh preserves the remote exit status (unlike Azure's run-command, which
# masks it), so the install script's own failure handling surfaces directly
# and no stdout sentinel check is needed.
run_via_gcloud_ssh() {
     ssh_cmd="$1"
     logaction "Running install via gcloud compute ssh"
     gcloud_scoped compute ssh "${VM_NAME}" \
          --zone "${ZONE}" \
          --command "${ssh_cmd}" >&3 2>&3
}

# =============================================================================
# GCE VM: identity + install
# =============================================================================

setup_gcp_vm() {
     if [ -z "${ZONE}" ] || [ -z "${VM_NAME}" ]; then usage; fi

     section "Reading GCE VM identity..."

     # Unlike the Azure script, no identity is ever assigned here: attaching a
     # service account to an existing GCE VM requires stopping it first, so a
     # VM without one is reported rather than mutated. VMs get the project's
     # default compute service account at creation unless opted out.
     SA_EMAIL=$(gcloud_scoped compute instances describe "${VM_NAME}" \
          --zone "${ZONE}" \
          --format 'value(serviceAccounts[0].email)' 2>/dev/null) ||
          die "cannot read VM ${VM_NAME} in zone ${ZONE} of project ${PROJECT} (check the name, zone, and project)"
     if [ -z "${SA_EMAIL}" ]; then
          die "no service account attached to ${VM_NAME}. Attach one (the VM must be stopped first):
  gcloud compute instances set-service-account ${VM_NAME} --zone ${ZONE} --service-account <sa-email>"
     fi
     log "Service account: ${SA_EMAIL}"

     # The unique ID is what the AWS trust policy conditions on (:sub and :aud).
     # The describe is not project-scoped: the email is globally unique and the
     # account may live in a different project than the VM.
     SA_UNIQUE_ID=$(gcloud iam service-accounts describe "${SA_EMAIL}" \
          --format 'value(uniqueId)' 2>/dev/null) ||
          die "cannot read service account ${SA_EMAIL} (needs iam.serviceAccounts.get on its project)"
     log "Service account unique ID: ${SA_UNIQUE_ID}"

     add_env CWAGENT_PLATFORM "${PLATFORM}"
     add_env CWAGENT_GCP_PROJECT "${PROJECT}"
     add_env CWAGENT_GCP_ZONE "${ZONE}"
     add_env CWAGENT_GCP_VM_NAME "${VM_NAME}"
     add_env CWAGENT_GCP_SA_UNIQUE_ID "${SA_UNIQUE_ID}"

     # No ARN yet: identity is done, emit the unique ID for the AWS trust step and stop.
     if [ -z "${ROLE_ARN}" ]; then
          print_await_arn
          return
     fi

     install_env="CWAGENT_CLOUD=gcp CWAGENT_AWS_ROLE_ARN=${ROLE_ARN} CWAGENT_AWS_REGION=${REGION}"
     INSTALL_CMD=$(linux_install_cmd "${install_env}")

     # Reachability probe, mirroring the EC2 path's SSM Online check: separates
     # "cannot SSH to the VM" (fall back to printing the command) from "install
     # failed" (die). The probe also performs gcloud's one-time SSH key setup.
     if gcloud_scoped compute ssh "${VM_NAME}" --zone "${ZONE}" --command true >/dev/null 2>&1; then
          section "Installing agent on ${VM_NAME}..."
          run_via_gcloud_ssh "${INSTALL_CMD}" || die "Install script failed on ${VM_NAME}"
          log "Agent installed on ${VM_NAME}"
          return
     fi
     logwarn "cannot reach ${VM_NAME} over SSH (check network access and compute.instances.osLogin/SSH key permissions)"

     printf '\n' >&3
     printf 'Done. Run the following on %s to install and start the agent:\n' "${VM_NAME}" >&3
     printf '\n' >&3
     printf '%s\n' "  ${INSTALL_CMD}" >&3
}

# =============================================================================
# GKE: identity + install
# =============================================================================

setup_gcp_gke() {
     if [ -z "${LOCATION}" ] || [ -z "${CLUSTER_NAME}" ]; then usage; fi

     section "Reading GKE cluster identity..."

     gcloud_scoped container clusters describe "${CLUSTER_NAME}" \
          --location "${LOCATION}" \
          --format 'value(name)' >/dev/null 2>&1 ||
          die "cannot read cluster ${CLUSTER_NAME} in location ${LOCATION} of project ${PROJECT} (check the name, location, and project)"

     # GKE serves the cluster's OIDC discovery natively at this URL (no enable
     # step, unlike AKS), keyed by "locations/" for zonal and regional clusters
     # alike, so the issuer is constructed rather than read off the cluster
     # (whose selfLink uses "zones/" for zonal clusters).
     GCP_OIDC_ISSUER="https://container.googleapis.com/v1/projects/${PROJECT}/locations/${LOCATION}/clusters/${CLUSTER_NAME}"
     log "OIDC issuer: ${GCP_OIDC_ISSUER}"

     add_env CWAGENT_PLATFORM "${PLATFORM}"
     add_env CWAGENT_GCP_PROJECT "${PROJECT}"
     add_env CWAGENT_GCP_LOCATION "${LOCATION}"
     add_env CWAGENT_K8S_CLUSTER_NAME "${CLUSTER_NAME}"
     add_env CWAGENT_GCP_OIDC_ISSUER "${GCP_OIDC_ISSUER}"

     # No ARN yet: identity is done, emit the issuer for the AWS trust step and stop.
     if [ -z "${ROLE_ARN}" ]; then
          print_await_arn
          return
     fi

     # Install directly when the toolchain is complete, otherwise print the
     # commands to run from a shell that has it. kubectl needs
     # gke-gcloud-auth-plugin to authenticate against GKE, so its absence is
     # treated like a missing kubectl.
     if command -v helm >/dev/null 2>&1 && command -v kubectl >/dev/null 2>&1 && command -v gke-gcloud-auth-plugin >/dev/null 2>&1; then
          section "Installing CloudWatch Observability Helm chart on ${CLUSTER_NAME}..."
          logaction "Configuring kubeconfig for ${CLUSTER_NAME}"
          gcloud_scoped container clusters get-credentials "${CLUSTER_NAME}" --location "${LOCATION}" >&3 2>&3

          logaction "Installing via Helm"
          # --force-update surfaces (not swallows) a stale-URL conflict if the
          # repo alias already points somewhere else.
          helm repo add --force-update aws-observability "${HELM_CHART_REPO}" >&3
          helm repo update >&3
          # Every agents[] entry is listed, not just the one being changed: --set replaces a whole
          # list element rather than merging into it, so setting agents[0] alone drops the chart's
          # default agents[1] and the cluster scraper is never created.
          helm upgrade --install amazon-cloudwatch-observability aws-observability/amazon-cloudwatch-observability \
               --set k8sMode=GKE \
               --set roleArn="${ROLE_ARN}" \
               --set region="${REGION}" \
               --set clusterName="${CLUSTER_NAME}" \
               --set containerInsights.enabled=false \
               --set containerLogs.enabled=false \
               --set otelContainerInsights.enabled=true \
               --set otelContainerInsights.logs.enabled=true \
               --set-string 'agents[0].name=cloudwatch-agent' \
               --set-string 'agents[0].config=default:otel' \
               --set-string 'agents[1].name=cloudwatch-agent-cluster-scraper' \
               --set-string 'agents[1].mode=deployment' \
               --set-string 'agents[1].config=default' \
               --namespace "${K8S_NAMESPACE}" \
               --create-namespace >&3
          log "Chart installed on ${CLUSTER_NAME}"
     else
          printf '\n' >&3
          printf 'Done. Install the Amazon CloudWatch Observability Helm chart (requires kubeconfig for %s):\n' "${CLUSTER_NAME}" >&3
          printf '\n' >&3
          {
               printf '%s\n' "  helm repo add aws-observability ${HELM_CHART_REPO}"
               printf '%s\n' "  helm repo update"
               printf '  helm upgrade --install amazon-cloudwatch-observability aws-observability/amazon-cloudwatch-observability \\\n'
               printf '    --set k8sMode=GKE \\\n'
               printf '    --set roleArn=%s \\\n' "${ROLE_ARN}"
               printf '    --set region=%s \\\n' "${REGION}"
               printf '    --set clusterName=%s \\\n' "${CLUSTER_NAME}"
               printf '    --set containerInsights.enabled=false \\\n'
               printf '    --set containerLogs.enabled=false \\\n'
               printf '    --set otelContainerInsights.enabled=true \\\n'
               printf '    --set otelContainerInsights.logs.enabled=true \\\n'
               printf "    --set-string 'agents[0].name=cloudwatch-agent' \\\\\n"
               printf "    --set-string 'agents[0].config=default:otel' \\\\\n"
               printf "    --set-string 'agents[1].name=cloudwatch-agent-cluster-scraper' \\\\\n"
               printf "    --set-string 'agents[1].mode=deployment' \\\\\n"
               printf "    --set-string 'agents[1].config=default' \\\\\n"
               printf '    --namespace %s \\\n' "${K8S_NAMESPACE}"
               printf '    --create-namespace\n'
          } >&3
     fi
}

main() {
     case "${1:-}" in -h | --help) usage 0 ;; esac

     # Interactive only on a real TTY and never under CWAGENT_EMIT_ENV (the
     # dispatcher eval-chains this script and cannot answer prompts).
     if [ -t 0 ] && ! is_true "${EMIT_ENV}" && [ -z "${PLATFORM}" ]; then
          interactive_setup
     fi

     case "${PLATFORM}" in
     gcp_vm | gcp_gke) ;;
     *) die "unsupported platform: ${PLATFORM:-<unset>} (valid: gcp_vm, gcp_gke)" ;;
     esac

     # Region is only needed for the install, so require it only when the ARN is set.
     if [ -n "${ROLE_ARN}" ]; then
          [ -n "${REGION}" ] || die "CWAGENT_AWS_REGION is required to install"
     fi

     check_prerequisites

     case "${PLATFORM}" in
     gcp_vm) setup_gcp_vm ;;
     gcp_gke) setup_gcp_gke ;;
     esac

     emit_env
}

main "$@"
