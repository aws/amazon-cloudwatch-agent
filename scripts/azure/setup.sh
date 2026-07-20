#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent - Azure setup (identity + install)
#
# Configures the Azure-side identity the agent uses and installs it:
#
#   azure_vm    assigns a system-assigned managed identity to the VM, then
#               pushes install.sh to it via "az vm run-command"
#               (requires: az)
#   azure_aks   enables the OIDC issuer and workload identity on the cluster
#               (an "az aks update" that can take several minutes), then
#               installs the CloudWatch Observability Helm chart when helm and
#               kubectl are present, otherwise prints the command (requires: az)
#
# Two modes, keyed on CWAGENT_AWS_ROLE_ARN:
#
#   Set: identity AND install in one run. Install can precede the AWS trust
#     setup and the agent retries with backoff until the trust is in place.
#
#   Unset: identity only, then stop (for anyone without the ARN yet). Run
#     aws/setup.sh to get the ARN, then rerun with it set. Identity is
#     idempotent, so the rerun just installs.
#
# Usage:
#   Interactive (TTY):
#     ./azure/setup.sh
#
#   Environment variables (piped/automated):
#     CWAGENT_PLATFORM=azure_aks \
#     CWAGENT_AWS_ROLE_ARN=arn:aws:iam::... \
#     CWAGENT_AWS_REGION=us-east-1 \
#     CWAGENT_AZURE_RESOURCE_GROUP=rg \
#     CWAGENT_K8S_CLUSTER_NAME=cluster \
#       ./azure/setup.sh
#
# Environment variables:
#   Common:
#     CWAGENT_PLATFORM              azure_vm | azure_aks
#     CWAGENT_AWS_ROLE_ARN          IAM role ARN, install runs too when set
#     CWAGENT_AWS_REGION            AWS region telemetry is sent to (required
#                                   for install)
#     CWAGENT_AZURE_RESOURCE_GROUP  Resource group
#     CWAGENT_EMIT_ENV              When set (1/true/yes/on), print eval-able KEY='value'
#                                   lines on stdout and route all logging to stderr
#   azure_vm:
#     CWAGENT_AZURE_VM_NAME         VM name
#   azure_aks:
#     CWAGENT_K8S_CLUSTER_NAME      Cluster name

set -eu

PLATFORM="${CWAGENT_PLATFORM:-}"
ROLE_ARN="${CWAGENT_AWS_ROLE_ARN:-}"
REGION="${CWAGENT_AWS_REGION:-}"
RESOURCE_GROUP="${CWAGENT_AZURE_RESOURCE_GROUP:-}"
VM_NAME="${CWAGENT_AZURE_VM_NAME:-}"
CLUSTER_NAME="${CWAGENT_K8S_CLUSTER_NAME:-}"
EMIT_ENV="${CWAGENT_EMIT_ENV:-}"
HELM_CHART_REPO="https://aws-observability.github.io/helm-charts"

# Where the VM fetches the install payload (install.sh / install.ps1) from.
SCRIPT_BASE_URL="https://raw.githubusercontent.com/aws/amazon-cloudwatch-agent/setup-scripts/scripts"
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
# every command must keep its own output off plain stdout. A stray line that
# looks like an assignment is indistinguishable from a real one.
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
  CWAGENT_PLATFORM=azure_aks CWAGENT_AWS_ROLE_ARN=arn:aws:iam::... CWAGENT_AWS_REGION=us-east-1 $0

Environment variables:
  Common:
    CWAGENT_PLATFORM              azure_vm | azure_aks
    CWAGENT_AWS_ROLE_ARN          IAM role ARN; when set, install runs too
    CWAGENT_AWS_REGION            AWS region (required for install)
    CWAGENT_AZURE_RESOURCE_GROUP  Resource group
    CWAGENT_EMIT_ENV              Print eval-able KEY='value' lines on stdout
  azure_vm:
    CWAGENT_AZURE_VM_NAME         VM name
  azure_aks:
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

# POSIX version compare: true when $1 >= $2 as major.minor.patch. Pure parameter
# expansion (no `sort -V`, a GNU/BSD extension) and length-agnostic, so "2.47"
# compares equal to "2.47.0". Missing minor/patch fields count as 0.
version_ge() {
     _a_maj=${1%%.*}
     _a_rest=${1#*.}
     _a_min=${_a_rest%%.*}
     _a_pat=${_a_rest#*.}
     _b_maj=${2%%.*}
     _b_rest=${2#*.}
     _b_min=${_b_rest%%.*}
     _b_pat=${_b_rest#*.}
     case "$1" in *.*.*) : ;; *.*) _a_pat=0 ;; *)
          _a_min=0
          _a_pat=0
          ;;
     esac
     case "$2" in *.*.*) : ;; *.*) _b_pat=0 ;; *)
          _b_min=0
          _b_pat=0
          ;;
     esac
     [ "$_a_maj" -ne "$_b_maj" ] && {
          [ "$_a_maj" -gt "$_b_maj" ]
          return
     }
     [ "$_a_min" -ne "$_b_min" ] && {
          [ "$_a_min" -gt "$_b_min" ]
          return
     }
     [ "$_a_pat" -ge "$_b_pat" ]
}

# ARN-unknown mode: identity is done but install waits on the AWS trust setup.
# Emitted to fd 3 so it never lands on stdout's CWAGENT_EMIT_ENV KEY='value' lines.
print_await_arn() {
     printf '\n' >&3
     log "Azure identity configured (install pending the IAM role ARN)"
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
     printf '  azure_vm    Azure VM\n' >&3
     printf '  azure_aks   AKS cluster\n' >&3
     ask "Platform:"
     read -r choice || die "no platform selected"
     case "${choice}" in
     azure_vm) PLATFORM=azure_vm ;;
     azure_aks) PLATFORM=azure_aks ;;
     *) die "invalid platform: ${choice}" ;;
     esac

     printf '\n' >&3
     case "${PLATFORM}" in
     azure_vm)
          prompt RESOURCE_GROUP "Resource group"
          prompt VM_NAME "VM name"
          ;;
     azure_aks)
          prompt RESOURCE_GROUP "Resource group"
          prompt CLUSTER_NAME "Cluster name"
          ;;
     esac
}

check_prerequisites() {
     command -v az >/dev/null 2>&1 || die "Azure CLI is required but not installed"
     AZ_CLI_VERSION=$(az version --query '"azure-cli"' -o tsv 2>/dev/null || echo "0.0.0")
     if ! version_ge "${AZ_CLI_VERSION}" "2.47.0"; then
          logwarn "Azure CLI ${AZ_CLI_VERSION} detected (2.47+ recommended for full functionality)"
     fi
     # One az account show for the liveness check, subscription id/name, and
     # tenant id (used later for the azure_vm identity emit).
     AZ_ACCOUNT=$(az account show --query "[id, name, tenantId]" -o tsv 2>/dev/null) ||
          die "Azure CLI not logged in (run 'az login')"
     AZ_SUB=$(printf '%s' "${AZ_ACCOUNT}" | cut -f1)
     AZ_NAME=$(printf '%s' "${AZ_ACCOUNT}" | cut -f2)
     AZ_TENANT=$(printf '%s' "${AZ_ACCOUNT}" | cut -f3)
     if [ -n "${AZ_NAME}" ]; then
          log "Azure subscription: ${AZ_SUB} (${AZ_NAME})"
     else
          log "Azure subscription: ${AZ_SUB}"
     fi
}

# =============================================================================
# Install payload builders (VM path)
# =============================================================================

# Emit the one-line command that fetches install.sh and runs it on a Linux
# target, with the given env assignments as a prefix. $1 = env assignments.
linux_install_cmd() {
     envs="$1"
     printf 'curl -fsSL %s/install.sh | %s sh' "${SCRIPT_BASE_URL}" "${envs}"
}

# Windows counterpart. The prelude + fetch are wrapped into one -EncodedCommand
# base64 payload (UTF-16LE) so the outer command needs no quoting through
# run-command. $1 = PowerShell env prelude (e.g. "$env:CWAGENT_CLOUD='azure'; ")
windows_install_cmd() {
     ps_prelude="$1"
     ps_script="${ps_prelude}Invoke-WebRequest -Uri ${SCRIPT_BASE_URL}/install.ps1 -OutFile \$env:TEMP\\cwagent-install.ps1; & \$env:TEMP\\cwagent-install.ps1"
     encoded=$(printf '%s' "${ps_script}" | iconv -f utf-8 -t utf-16le 2>/dev/null | base64 | tr -d '\n')
     [ -n "${encoded}" ] || return 1
     printf 'powershell -NoProfile -EncodedCommand %s' "${encoded}"
}

# Run a command on an Azure VM via run-command. $1 = command ID
# (RunShellScript | RunPowerShellScript), $2 = command.
run_via_az() {
     az_command_id="$1"
     az_cmd="$2"
     RUN_MESSAGE=""

     logaction "Running install via az vm run-command"
     RUN_RESULT=$(az vm run-command invoke \
          --resource-group "${RESOURCE_GROUP}" \
          --name "${VM_NAME}" \
          --command-id "${az_command_id}" \
          --scripts "${az_cmd}" \
          -o json)

     # run-command returns one of two shapes: older builds emit separate
     # ComponentStatus/StdOut|StdErr entries, current ones return a single entry
     # whose message embeds both as "[stdout]\n...\n\n[stderr]\n...". Try the
     # split entries first, then fall back to splitting the combined message.
     if command -v jq >/dev/null 2>&1; then
          STDOUT=$(printf '%s' "${RUN_RESULT}" | jq -r '.value[] | select(.code == "ComponentStatus/StdOut/succeeded") | .message')
          STDERR=$(printf '%s' "${RUN_RESULT}" | jq -r '.value[] | select(.code == "ComponentStatus/StdErr/succeeded") | .message')
          if [ -z "${STDOUT}" ] && [ -z "${STDERR}" ]; then
               RUN_MESSAGE=$(printf '%s' "${RUN_RESULT}" | jq -r 'first(.value[] | select(.message != null) | .message) // ""')
          fi
     else
          STDOUT=$(printf '%s' "${RUN_RESULT}" | grep -A1 '"ComponentStatus/StdOut/succeeded"' | tail -1 | sed 's/.*"message": "//;s/"$//')
          STDERR=$(printf '%s' "${RUN_RESULT}" | grep -A1 '"ComponentStatus/StdErr/succeeded"' | tail -1 | sed 's/.*"message": "//;s/"$//')
          if [ -z "${STDOUT}" ] && [ -z "${STDERR}" ]; then
               RUN_MESSAGE=$(printf '%s' "${RUN_RESULT}" | grep '"message":' | head -1 | sed 's/.*"message": "//;s/"$//')
               # Turn the JSON-escaped newlines back into real ones so the split works.
               RUN_MESSAGE=$(printf '%b' "${RUN_MESSAGE}")
          fi
     fi

     if [ -n "${RUN_MESSAGE:-}" ]; then
          STDOUT=$(printf '%s' "${RUN_MESSAGE}" | sed -n '/^\[stdout\]$/,/^\[stderr\]$/p' | sed '1d;$d')
          STDERR=$(printf '%s' "${RUN_MESSAGE}" | sed -n '/^\[stderr\]$/,$p' | sed '1d')
     fi

     # run-command masks the exit code and the agent logs benign errors to
     # stderr, so key success off the install script's stdout sentinel (printed
     # only after it asserts the agent is running). Show stdout either way, and
     # add the stderr transcript only on failure, for diagnosis.
     if [ -n "${STDOUT}" ]; then printf '%s\n' "${STDOUT}" >&3; fi

     if ! printf '%s' "${STDOUT}" | grep -q 'Amazon CloudWatch Agent installed and running\.'; then
          [ -n "${STDERR}" ] && printf '%s\n' "${STDERR}" >&2
          die "Install script failed on ${VM_NAME}"
     fi
}

# =============================================================================
# Azure VM: identity + install
# =============================================================================

setup_azure_vm() {
     if [ -z "${RESOURCE_GROUP}" ] || [ -z "${VM_NAME}" ]; then usage; fi

     section "Configuring Azure VM identity..."

     # One az vm show for both fields: the identity (to decide whether to assign
     # one) and the OS type (to pick the install payload). A tsv list projection
     # returns them tab-separated on one line, in query order.
     VM_INFO=$(az vm show \
          --resource-group "${RESOURCE_GROUP}" \
          --name "${VM_NAME}" \
          --query "[identity.principalId, storageProfile.osDisk.osType]" -o tsv 2>/dev/null || true)
     IDENTITY=$(printf '%s' "${VM_INFO}" | cut -f1)
     VM_OS=$(printf '%s' "${VM_INFO}" | cut -f2)

     if [ -n "${IDENTITY}" ] && [ "${IDENTITY}" != "None" ]; then
          log "Managed identity enabled on ${VM_NAME}"
     else
          logaction "Enabling managed identity (this may take a few minutes)"
          az vm identity assign \
               --resource-group "${RESOURCE_GROUP}" \
               --name "${VM_NAME}" \
               --output none
          log "Managed identity enabled on ${VM_NAME}"
     fi

     TENANT_ID="${AZ_TENANT}"
     log "Tenant: ${TENANT_ID}"

     add_env CWAGENT_PLATFORM "${PLATFORM}"
     add_env CWAGENT_AZURE_TENANT_ID "${TENANT_ID}"

     # No ARN yet: identity is done, emit the tenant ID for the AWS trust step and stop.
     if [ -z "${ROLE_ARN}" ]; then
          print_await_arn
          return
     fi

     section "Installing agent on ${VM_NAME}..."
     if [ "${VM_OS}" = "Windows" ]; then
          ps_prelude="\$env:CWAGENT_CLOUD='azure'; \$env:CWAGENT_AWS_ROLE_ARN='${ROLE_ARN}'; \$env:CWAGENT_AWS_REGION='${REGION}'; "
          if INSTALL_CMD=$(windows_install_cmd "${ps_prelude}"); then
               run_via_az "RunPowerShellScript" "${INSTALL_CMD}"
               log "Agent installed on ${VM_NAME}"
               return
          fi
     else
          install_env="CWAGENT_CLOUD=azure CWAGENT_AWS_ROLE_ARN=${ROLE_ARN} CWAGENT_AWS_REGION=${REGION}"
          if INSTALL_CMD=$(linux_install_cmd "${install_env}"); then
               run_via_az "RunShellScript" "${INSTALL_CMD}"
               log "Agent installed on ${VM_NAME}"
               return
          fi
     fi
     logwarn "could not build the install command (iconv is required for Windows targets)"

     printf '\n' >&3
     printf 'Done. Run the following on %s to install and start the agent:\n' "${VM_NAME}" >&3
     printf '\n' >&3
     if [ "${VM_OS}" = "Windows" ]; then
          printf '%s\n' "  # PowerShell, as Administrator:" >&3
          printf '%s\n' "  \$env:CWAGENT_CLOUD='azure'; \$env:CWAGENT_AWS_ROLE_ARN='${ROLE_ARN}'; \$env:CWAGENT_AWS_REGION='${REGION}'" >&3
          printf '%s\n' "  Invoke-WebRequest -Uri ${SCRIPT_BASE_URL}/install.ps1 -OutFile \$env:TEMP\\install.ps1; & \$env:TEMP\\install.ps1" >&3
     else
          printf '%s\n' "  curl -fsSL ${SCRIPT_BASE_URL}/install.sh | sudo ${install_env} sh" >&3
     fi
}

# =============================================================================
# Azure AKS: identity + install
# =============================================================================

setup_azure_aks() {
     if [ -z "${RESOURCE_GROUP}" ] || [ -z "${CLUSTER_NAME}" ]; then usage; fi

     section "Configuring AKS cluster..."

     # One az aks show for both the enabled flag and the issuer URL (present when
     # already enabled). Only re-fetch the URL after enabling it ourselves.
     AKS_INFO=$(az aks show \
          --resource-group "${RESOURCE_GROUP}" \
          --name "${CLUSTER_NAME}" \
          --query "[oidcIssuerProfile.enabled, oidcIssuerProfile.issuerUrl]" -o tsv 2>/dev/null || true)
     OIDC_ENABLED=$(printf '%s' "${AKS_INFO}" | cut -f1)
     OIDC_ISSUER=$(printf '%s' "${AKS_INFO}" | cut -f2)

     # az -o tsv renders a JSON boolean via Python str(), i.e. "True"/"False",
     # so match case-insensitively rather than against a lowercase "true".
     if [ "$(printf '%s' "${OIDC_ENABLED}" | tr '[:upper:]' '[:lower:]')" = "true" ]; then
          log "OIDC issuer and workload identity enabled"
     else
          logaction "Enabling OIDC issuer and workload identity (this may take a few minutes)"
          az aks update \
               --resource-group "${RESOURCE_GROUP}" \
               --name "${CLUSTER_NAME}" \
               --enable-oidc-issuer \
               --enable-workload-identity \
               --output none
          # Re-fetch the now-populated issuer URL. No "|| true" here (unlike the
          # probe above): we just changed cluster state, so a failure should
          # surface loudly rather than leave OIDC_ISSUER empty.
          OIDC_ISSUER=$(az aks show \
               --resource-group "${RESOURCE_GROUP}" \
               --name "${CLUSTER_NAME}" \
               --query "oidcIssuerProfile.issuerUrl" -o tsv)
     fi

     log "OIDC issuer: ${OIDC_ISSUER}"

     add_env CWAGENT_PLATFORM "${PLATFORM}"
     add_env CWAGENT_AZURE_OIDC_ISSUER "${OIDC_ISSUER}"

     # No ARN yet: identity is done, emit the OIDC issuer for the AWS trust step and stop.
     if [ -z "${ROLE_ARN}" ]; then
          print_await_arn
          return
     fi

     # Install directly when helm and kubectl are available, otherwise print the
     # commands to run from a shell that has them.
     if command -v helm >/dev/null 2>&1 && command -v kubectl >/dev/null 2>&1; then
          section "Installing CloudWatch Observability Helm chart on ${CLUSTER_NAME}..."
          logaction "Configuring kubeconfig for ${CLUSTER_NAME}"
          az aks get-credentials --resource-group "${RESOURCE_GROUP}" --name "${CLUSTER_NAME}" --overwrite-existing >&3

          logaction "Installing via Helm"
          # --force-update surfaces (not swallows) a stale-URL conflict if the
          # repo alias already points somewhere else.
          helm repo add --force-update aws-observability "${HELM_CHART_REPO}" >&3
          helm repo update >&3
          # Every agents[] entry is listed, not just the one being changed: --set replaces a whole
          # list element rather than merging into it, so setting agents[0] alone drops the chart's
          # default agents[1] and the cluster scraper is never created.
          helm upgrade --install amazon-cloudwatch-observability aws-observability/amazon-cloudwatch-observability \
               --set k8sMode=AKS \
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
               printf '    --set k8sMode=AKS \\\n'
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
     azure_vm | azure_aks) ;;
     *) die "unsupported platform: ${PLATFORM:-<unset>} (valid: azure_vm, azure_aks)" ;;
     esac

     # Region is only needed for the install, so require it only when the ARN is set.
     if [ -n "${ROLE_ARN}" ]; then
          [ -n "${REGION}" ] || die "CWAGENT_AWS_REGION is required to install"
     fi

     check_prerequisites

     case "${PLATFORM}" in
     azure_vm) setup_azure_vm ;;
     azure_aks) setup_azure_aks ;;
     esac

     emit_env
}

main "$@"
