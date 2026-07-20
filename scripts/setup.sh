#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent — onboarding setup
#
# Provisions the credentials and platform resources the CloudWatch Agent
# needs to send OpenTelemetry (OTLP) telemetry to CloudWatch, then prints
# the install and configuration commands to run. Safe to re-run
# (idempotent).
#
# Usage:
#   ./setup.sh                          Interactive wizard (TTY)
#
#   Or via environment variables (piped/automated):
#   CWAGENT_PLATFORM=aws_ec2 CWAGENT_AWS_INSTANCE_ID=i-123 ./setup.sh
#
# Interactive mode prompts for platform and required options. For EC2,
# it discovers the existing instance profile and role, using them as
# defaults. If the instance already has a role, setup attaches the
# required policy to it without creating new IAM resources.
#
# Environment variables:
#   CWAGENT_PLATFORM                      aws_ec2 | aws_ecs | aws_eks | azure_vm | azure_aks
#   CWAGENT_IAM_ROLE_NAME                 IAM role name (default: CloudWatchAgentServerRole)
#   CWAGENT_AWS_REGION                    AWS region
#   CWAGENT_SKIP_INSTALL                  Skip installation (print command instead)
#   CWAGENT_INSTALL_URL                   Override package/artifact download URL (pre-release testing)
#   CWAGENT_INSTALL_SCRIPT_URL            URL hosting install.sh/install.ps1 (default: use local copy next to this script)
#   CWAGENT_HELM_REPO                     Override Helm chart repository (AKS)
#   CWAGENT_IMAGE                         Override container image (ECS)
#
#   EC2:
#   CWAGENT_AWS_INSTANCE_ID               EC2 instance ID
#   CWAGENT_AWS_REPLACE_INSTANCE_PROFILE  Set to "true" to replace existing instance profile
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
#   CWAGENT_K8S_NAMESPACE                 Namespace (default: amazon-cloudwatch)

set -eu

ROLE_NAME="${CWAGENT_IAM_ROLE_NAME:-CloudWatchAgentServerRole}"
PLATFORM="${CWAGENT_PLATFORM:-}"
INSTANCE_ID="${CWAGENT_AWS_INSTANCE_ID:-}"
CLUSTER_NAME="${CWAGENT_K8S_CLUSTER_NAME:-}"
REGION="${CWAGENT_AWS_REGION:-}"
DETECTED_REGION="${AWS_REGION:-${AWS_DEFAULT_REGION:-}}"
NAMESPACE="${CWAGENT_K8S_NAMESPACE:-}"
RESOURCE_GROUP="${CWAGENT_AZURE_RESOURCE_GROUP:-}"
VM_NAME="${CWAGENT_AZURE_VM_NAME:-}"
REPLACE_INSTANCE_PROFILE="${CWAGENT_AWS_REPLACE_INSTANCE_PROFILE:-}"
ECS_LAUNCH_TYPE="${CWAGENT_AWS_ECS_LAUNCH_TYPE:-}"
SKIP_INSTALL="${CWAGENT_SKIP_INSTALL:-}"
INSTALL_URL="${CWAGENT_INSTALL_URL:-}"
INSTALL_SCRIPT_URL="${CWAGENT_INSTALL_SCRIPT_URL:-}"
CONTAINER_IMAGE="${CWAGENT_IMAGE:-public.ecr.aws/cloudwatch-agent/cloudwatch-agent:latest}"
DOWNLOAD_BASE="https://amazoncloudwatch-agent.s3.amazonaws.com"
CTL="/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl"
CTL_PS1="& \"\${Env:ProgramFiles}\\Amazon\\AmazonCloudWatchAgent\\amazon-cloudwatch-agent-ctl.ps1\""
HELM_CHART_REPO="${CWAGENT_HELM_REPO:-https://aws-observability.github.io/helm-charts}"
EKS_ADDON_NAME="amazon-cloudwatch-observability"
SCRIPT_DIR=$(dirname "$0")

usage() {
     cat >&2 <<EOF
Usage:
  $0                    Interactive wizard (TTY)

  Or via environment variables:
  CWAGENT_PLATFORM=aws_ec2 CWAGENT_AWS_INSTANCE_ID=i-123 $0

Environment variables:
  CWAGENT_PLATFORM                        aws_ec2 | aws_ecs | aws_eks | azure_vm | azure_aks
  CWAGENT_IAM_ROLE_NAME                   IAM role name (default: CloudWatchAgentServerRole)
  CWAGENT_AWS_REGION                      AWS region
  CWAGENT_SKIP_INSTALL                    Skip installation (print command instead)
  CWAGENT_INSTALL_URL                     Override package/artifact download URL
  CWAGENT_INSTALL_SCRIPT_URL              URL hosting install.sh/install.ps1
  CWAGENT_HELM_REPO                       Override Helm chart repository (AKS)
  CWAGENT_IMAGE                           Override container image (ECS)

  EC2:
  CWAGENT_AWS_INSTANCE_ID                 EC2 instance ID
  CWAGENT_AWS_REPLACE_INSTANCE_PROFILE    Replace existing instance profile

  ECS:
  CWAGENT_AWS_ECS_LAUNCH_TYPE             fargate | ec2

  Azure:
  CWAGENT_AZURE_RESOURCE_GROUP            Resource group
  CWAGENT_AZURE_VM_NAME                   VM name (azure_vm only)

  Kubernetes (EKS, AKS):
  CWAGENT_K8S_CLUSTER_NAME                Cluster name
  CWAGENT_K8S_NAMESPACE                   Namespace (default: amazon-cloudwatch)
EOF
     exit 1
}

# =============================================================================
# Output helpers
# =============================================================================

section() { printf '\n%s\n' "$1"; }
if [ -t 1 ]; then
     log() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
     logaction() { printf '  \033[33m+\033[0m %s\n' "$1"; }
     logwarn() { printf '  \033[33m!\033[0m %s\n' "$1"; }
     die() {
          printf '  \033[31m✗\033[0m %s\n' "$1" >&2
          exit 1
     }
     ask() { printf '\033[1m▸ %s\033[0m ' "$1"; }
else
     log() { printf '  ✓ %s\n' "$1"; }
     logaction() { printf '  + %s\n' "$1"; }
     logwarn() { printf '  ! %s\n' "$1"; }
     die() {
          printf '  ✗ %s\n' "$1" >&2
          exit 1
     }
     ask() { printf '▸ %s ' "$1"; }
fi

# =============================================================================
# Prerequisite checks
# =============================================================================

check_prerequisites() {
     command -v aws >/dev/null 2>&1 || die "AWS CLI is required but not installed"
     AWS_CLI_VERSION=$(aws --version 2>&1 | grep -o 'aws-cli/[0-9.]*' | cut -d/ -f2)
     if [ "$(printf '%s\n' "2.22.0" "${AWS_CLI_VERSION}" | sort -V | head -1)" != "2.22.0" ]; then
          logwarn "AWS CLI ${AWS_CLI_VERSION} detected — 2.22+ recommended for full functionality"
     fi
     AWS_IDENTITY=$(aws sts get-caller-identity --query '[Account, Arn]' --output text 2>&1) || die "AWS credentials not configured (run 'aws configure' or set AWS_PROFILE)"
     AWS_ACCOUNT=$(printf '%s' "${AWS_IDENTITY}" | cut -f1)
     AWS_ARN=$(printf '%s' "${AWS_IDENTITY}" | cut -f2)
     AWS_ALIAS=$(aws iam list-account-aliases --query 'AccountAliases[0]' --output text 2>/dev/null || true)
     if [ -n "${AWS_ALIAS}" ] && [ "${AWS_ALIAS}" != "None" ]; then
          log "AWS account: ${AWS_ACCOUNT} (${AWS_ALIAS})"
     else
          log "AWS account: ${AWS_ACCOUNT}"
     fi
     log "AWS identity: ${AWS_ARN}"

     case "${PLATFORM}" in
     azure_vm | azure_aks)
          command -v az >/dev/null 2>&1 || die "Azure CLI is required but not installed"
          AZ_CLI_VERSION=$(az version --query '"azure-cli"' -o tsv 2>/dev/null || echo "0.0.0")
          if [ "$(printf '%s\n' "2.47.0" "${AZ_CLI_VERSION}" | sort -V | head -1)" != "2.47.0" ]; then
               logwarn "Azure CLI ${AZ_CLI_VERSION} detected — 2.47+ recommended for full functionality"
          fi
          az account show >/dev/null 2>&1 || die "Azure CLI not logged in (run 'az login')"
          AZ_SUB=$(az account show --query id -o tsv)
          AZ_NAME=$(az account show --query name -o tsv 2>/dev/null || true)
          if [ -n "${AZ_NAME}" ]; then
               log "Azure subscription: ${AZ_SUB} (${AZ_NAME})"
          else
               log "Azure subscription: ${AZ_SUB}"
          fi
          ;;
     esac

     case "${PLATFORM}" in
     azure_aks)
          if [ "${SKIP_INSTALL}" != "true" ]; then
               command -v helm >/dev/null 2>&1 || die "Helm is required for AKS installs (or set CWAGENT_SKIP_INSTALL=true)"
               command -v kubectl >/dev/null 2>&1 || die "kubectl is required for AKS installs (or set CWAGENT_SKIP_INSTALL=true)"
          fi
          ;;
     esac
}

# =============================================================================
# Shared helpers
# =============================================================================

ensure_iam_role() {
     new_statement="$1"
     full_policy="{\"Version\":\"2012-10-17\",\"Statement\":[${new_statement}]}"

     if ! aws iam get-role --role-name "${ROLE_NAME}" >/dev/null 2>&1; then
          logaction "Creating IAM role ${ROLE_NAME}"
          aws iam create-role \
               --role-name "${ROLE_NAME}" \
               --assume-role-policy-document "${full_policy}" \
               >/dev/null
          return
     fi

     existing=$(aws iam get-role --role-name "${ROLE_NAME}" \
          --query 'Role.AssumeRolePolicyDocument' --output json)

     if ! command -v jq >/dev/null 2>&1; then
          die "jq is required to verify and update the trust policy on existing role ${ROLE_NAME} (install jq or use CWAGENT_IAM_ROLE_NAME to create a separate role)"
     fi

     new_principal=$(printf '%s' "${new_statement}" | jq -r \
          '(.Principal | if type == "object" then to_entries[0].value else . end)')

     already_present=$(printf '%s' "${existing}" | jq -r \
          --arg principal "${new_principal}" \
          '[.Statement[] | .Principal | if type == "object" then to_entries[].value else . end] | if index($principal) then "yes" else "no" end')

     if [ "${already_present}" = "yes" ]; then
          log "IAM role ${ROLE_NAME} trust policy up to date"
          return
     fi

     logaction "Merging trust statement into ${ROLE_NAME}"
     merged=$(printf '%s' "${existing}" | jq \
          --argjson stmt "${new_statement}" \
          '.Statement += [$stmt]')

     aws iam update-assume-role-policy \
          --role-name "${ROLE_NAME}" \
          --policy-document "${merged}"
}

attach_permissions_policy() {
     aws iam attach-role-policy \
          --role-name "${ROLE_NAME}" \
          --policy-arn arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy 2>/dev/null || true
     log "Managed policy CloudWatchAgentServerPolicy attached"
}

# Transaction Search must be enabled for OTLP traces to reach X-Ray.
ensure_transaction_search() {
     TRACE_DEST=$(aws xray get-trace-segment-destination --region "${REGION}" --query 'Destination' --output text 2>/dev/null) || TRACE_DEST_RC=$?
     if [ -n "${TRACE_DEST_RC:-}" ]; then
          logwarn "Could not verify Transaction Search status — must be enabled to send OTLP traces to X-Ray:"
          logwarn "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Enable-TransactionSearch.html"
     elif [ "${TRACE_DEST}" != "CloudWatchLogs" ]; then
          logaction "Enabling Transaction Search (required for OTLP traces)"
          if aws xray update-trace-segment-destination --destination CloudWatchLogs --region "${REGION}" >/dev/null 2>&1; then
               log "Transaction Search enabled"
          else
               logwarn "Failed to enable Transaction Search — must be enabled to send OTLP traces to X-Ray:"
               logwarn "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Enable-TransactionSearch.html"
          fi
     fi
}

# Print install instructions for a Linux VM. Uses CWAGENT_INSTALL_URL when
# set (pre-release artifacts); otherwise the released S3 packages.
print_linux_install() {
     if [ -n "${INSTALL_URL}" ]; then
          echo "  # Download and install the agent package:"
          echo "  curl -fsSL '${INSTALL_URL}' -o /tmp/\$(basename '${INSTALL_URL}')"
          case "${INSTALL_URL}" in
          *.rpm) echo "  sudo rpm -Uvh /tmp/\$(basename '${INSTALL_URL}')" ;;
          *.deb) echo "  sudo dpkg -i /tmp/\$(basename '${INSTALL_URL}')" ;;
          *) echo "  # Install the downloaded artifact for your distribution" ;;
          esac
     else
          echo "  # Amazon Linux / RHEL:"
          echo "  sudo rpm -Uvh ${DOWNLOAD_BASE}/amazon_linux/\$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')/latest/amazon-cloudwatch-agent.rpm"
          echo ""
          echo "  # Ubuntu / Debian:"
          echo "  curl -fsSL ${DOWNLOAD_BASE}/ubuntu/\$(dpkg --print-architecture)/latest/amazon-cloudwatch-agent.deb -o /tmp/amazon-cloudwatch-agent.deb"
          echo "  sudo dpkg -i /tmp/amazon-cloudwatch-agent.deb"
     fi
}

print_windows_install() {
     if [ -n "${INSTALL_URL}" ]; then
          echo "  # Download and install the agent package:"
          echo "  Invoke-WebRequest -Uri '${INSTALL_URL}' -OutFile \$env:TEMP\\amazon-cloudwatch-agent.msi"
          echo "  msiexec /i \$env:TEMP\\amazon-cloudwatch-agent.msi"
     else
          echo "  Invoke-WebRequest -Uri ${DOWNLOAD_BASE}/windows/amd64/latest/amazon-cloudwatch-agent.msi -OutFile \$env:TEMP\\amazon-cloudwatch-agent.msi"
          echo "  msiexec /i \$env:TEMP\\amazon-cloudwatch-agent.msi"
     fi
}

b64encode() {
     base64 -w0 "$1" 2>/dev/null || base64 "$1" | tr -d '\n'
}

# Build a single-line command that runs install.sh on a Linux target with the
# given env assignments. Uses the hosted script when CWAGENT_INSTALL_SCRIPT_URL
# is set; otherwise embeds the local copy (base64 keeps the payload free of
# characters that need escaping in SSM/run-command JSON). Returns 1 if neither
# source is available.
linux_install_cmd() {
     envs="$1"
     if [ -n "${INSTALL_SCRIPT_URL}" ]; then
          printf 'curl -fsSL %s/install.sh | %s sh' "${INSTALL_SCRIPT_URL}" "${envs}"
     elif [ -f "${SCRIPT_DIR}/install.sh" ]; then
          printf 'echo %s | base64 -d > /tmp/cwagent-install.sh && %s sh /tmp/cwagent-install.sh' \
               "$(b64encode "${SCRIPT_DIR}/install.sh")" "${envs}"
     else
          return 1
     fi
}

# Build a single-line PowerShell command that runs install.ps1 on a Windows
# target. The env-var prelude and script are wrapped into one base64 payload
# executed via -EncodedCommand (UTF-16LE), so the outer command has no quoting.
# $1 = PowerShell env prelude (e.g. "$env:CWAGENT_CLOUD='azure'; ")
windows_install_cmd() {
     ps_prelude="$1"
     if [ -n "${INSTALL_SCRIPT_URL}" ]; then
          ps_script="${ps_prelude}Invoke-WebRequest -Uri ${INSTALL_SCRIPT_URL}/install.ps1 -OutFile \$env:TEMP\\cwagent-install.ps1; & \$env:TEMP\\cwagent-install.ps1"
     elif [ -f "${SCRIPT_DIR}/install.ps1" ]; then
          ps_script="${ps_prelude}$(cat "${SCRIPT_DIR}/install.ps1")"
     else
          return 1
     fi
     command -v iconv >/dev/null 2>&1 || return 1
     printf 'powershell -NoProfile -EncodedCommand %s' \
          "$(printf '%s' "${ps_script}" | iconv -f utf-8 -t utf-16le | base64 | tr -d '\n')"
}

# Run a command on an EC2 instance via SSM. $1 = SSM document, $2 = command.
run_via_ssm() {
     ssm_doc="$1"
     ssm_cmd="$2"

     logaction "Running install via SSM"
     COMMAND_ID=$(aws ssm send-command \
          --instance-ids "${INSTANCE_ID}" \
          --document-name "${ssm_doc}" \
          --parameters "commands=[\"${ssm_cmd}\"]" \
          --region "${REGION}" --query 'Command.CommandId' --output text)

     aws ssm wait command-executed \
          --command-id "${COMMAND_ID}" \
          --instance-id "${INSTANCE_ID}" --region "${REGION}" 2>/dev/null || true

     SSM_OUTPUT=$(aws ssm get-command-invocation \
          --command-id "${COMMAND_ID}" \
          --instance-id "${INSTANCE_ID}" \
          --region "${REGION}" --query 'StandardOutputContent' --output text)
     SSM_STATUS_DETAIL=$(aws ssm get-command-invocation \
          --command-id "${COMMAND_ID}" \
          --instance-id "${INSTANCE_ID}" \
          --region "${REGION}" --query 'StatusDetails' --output text)

     echo "${SSM_OUTPUT}"
     if [ "${SSM_STATUS_DETAIL}" != "Success" ]; then
          aws ssm get-command-invocation \
               --command-id "${COMMAND_ID}" \
               --instance-id "${INSTANCE_ID}" \
               --region "${REGION}" --query 'StandardErrorContent' --output text >&2
          die "SSM command finished with status: ${SSM_STATUS_DETAIL}"
     fi
}

# Run a command on an Azure VM via run-command. $1 = command ID
# (RunShellScript | RunPowerShellScript), $2 = command.
run_via_az() {
     az_command_id="$1"
     az_cmd="$2"

     logaction "Running install via az vm run-command"
     RUN_RESULT=$(az vm run-command invoke \
          --resource-group "${RESOURCE_GROUP}" \
          --name "${VM_NAME}" \
          --command-id "${az_command_id}" \
          --scripts "${az_cmd}" \
          -o json)

     if command -v jq >/dev/null 2>&1; then
          STDOUT=$(printf '%s' "${RUN_RESULT}" | jq -r '.value[] | select(.code == "ComponentStatus/StdOut/succeeded") | .message')
          STDERR=$(printf '%s' "${RUN_RESULT}" | jq -r '.value[] | select(.code == "ComponentStatus/StdErr/succeeded") | .message')
     else
          STDOUT=$(printf '%s' "${RUN_RESULT}" | grep -A1 '"ComponentStatus/StdOut/succeeded"' | tail -1 | sed 's/.*"message": "//;s/"$//')
          STDERR=$(printf '%s' "${RUN_RESULT}" | grep -A1 '"ComponentStatus/StdErr/succeeded"' | tail -1 | sed 's/.*"message": "//;s/"$//')
     fi

     if [ -n "${STDOUT}" ]; then echo "${STDOUT}"; fi

     if [ -n "${STDERR}" ]; then
          printf '%s\n' "${STDERR}" >&2
          die "Install script failed on ${VM_NAME}"
     fi
}

# =============================================================================
# AWS EC2
# =============================================================================

setup_aws_ec2() {
     if [ -z "${INSTANCE_ID}" ]; then usage; fi

     PROFILE_NAME=""

     # --- discover existing profile from instance ---
     if [ -z "${CURRENT_PROFILE_ARN:-}" ]; then
          CURRENT_PROFILE_ARN=$(aws ec2 describe-iam-instance-profile-associations \
               --filters "Name=instance-id,Values=${INSTANCE_ID}" "Name=state,Values=associated" \
               --query 'IamInstanceProfileAssociations[0].IamInstanceProfile.Arn' \
               --region "${REGION}" --output text 2>/dev/null || true)
     fi

     if [ -n "${CURRENT_PROFILE_ARN}" ] && [ "${CURRENT_PROFILE_ARN}" != "None" ]; then
          PROFILE_NAME="${CURRENT_PROFILE_ARN##*/}"
          EXISTING_ROLE=$(aws iam get-instance-profile \
               --instance-profile-name "${PROFILE_NAME}" \
               --query 'InstanceProfile.Roles[0].RoleName' --output text 2>/dev/null || true)

          if [ -z "${EXISTING_ROLE}" ] || [ "${EXISTING_ROLE}" = "None" ]; then
               die "Instance profile ${PROFILE_NAME} has no role attached"
          fi

          if [ "${EXISTING_ROLE}" = "${ROLE_NAME}" ]; then
               section "Using existing instance profile..."
               log "Instance profile ${PROFILE_NAME} attached to ${INSTANCE_ID}"
               log "Role: ${EXISTING_ROLE}"
               attach_permissions_policy
          else
               # User chose a different role than what's on the profile
               if [ -t 0 ]; then
                    ask "Instance has profile ${PROFILE_NAME} (role: ${EXISTING_ROLE}). Replace with role ${ROLE_NAME}? [y/N]"
                    read -r answer
                    case "${answer}" in [yY]*) REPLACE_INSTANCE_PROFILE="true" ;; *) die "cannot proceed — role mismatch" ;; esac
               elif [ "${REPLACE_INSTANCE_PROFILE}" != "true" ]; then
                    die "Instance ${INSTANCE_ID} has profile ${PROFILE_NAME} with role ${EXISTING_ROLE} — set CWAGENT_AWS_REPLACE_INSTANCE_PROFILE=true to replace it"
               fi

               if [ "${REPLACE_INSTANCE_PROFILE}" = "true" ]; then
                    section "Configuring IAM role..."
                    ensure_iam_role '{
          "Effect": "Allow",
          "Principal": { "Service": "ec2.amazonaws.com" },
          "Action": "sts:AssumeRole"
        }'
                    attach_permissions_policy
                    section "Replacing instance profile..."
                    ASSOC_ID=$(aws ec2 describe-iam-instance-profile-associations \
                         --filters "Name=instance-id,Values=${INSTANCE_ID}" "Name=state,Values=associated" \
                         --query 'IamInstanceProfileAssociations[0].AssociationId' --region "${REGION}" --output text)
                    logaction "Replacing instance profile (was ${PROFILE_NAME})"
                    aws ec2 disassociate-iam-instance-profile --association-id "${ASSOC_ID}" --region "${REGION}" >/dev/null
                    PROFILE_NAME="${ROLE_NAME}"
                    if ! aws iam get-instance-profile --instance-profile-name "${PROFILE_NAME}" >/dev/null 2>&1; then
                         logaction "Creating instance profile ${PROFILE_NAME}"
                         aws iam create-instance-profile --instance-profile-name "${PROFILE_NAME}" >/dev/null
                         aws iam add-role-to-instance-profile \
                              --instance-profile-name "${PROFILE_NAME}" --role-name "${ROLE_NAME}" >/dev/null
                         logaction "Waiting for propagation..."
                         sleep 10
                    fi
                    aws ec2 associate-iam-instance-profile \
                         --instance-id "${INSTANCE_ID}" \
                         --iam-instance-profile Name="${PROFILE_NAME}" --region "${REGION}" >/dev/null
               fi
          fi
     else
          # No profile attached — create and attach
          PROFILE_NAME="${ROLE_NAME}"

          section "Configuring IAM role..."
          ensure_iam_role '{
      "Effect": "Allow",
      "Principal": { "Service": "ec2.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }'
          attach_permissions_policy

          section "Configuring instance profile..."
          if aws iam get-instance-profile --instance-profile-name "${PROFILE_NAME}" >/dev/null 2>&1; then
               log "Instance profile ${PROFILE_NAME} exists"
          else
               logaction "Creating instance profile ${PROFILE_NAME}"
               aws iam create-instance-profile \
                    --instance-profile-name "${PROFILE_NAME}" >/dev/null
               aws iam add-role-to-instance-profile \
                    --instance-profile-name "${PROFILE_NAME}" \
                    --role-name "${ROLE_NAME}" >/dev/null
               logaction "Waiting for propagation..."
               sleep 10
          fi

          logaction "Associating instance profile with ${INSTANCE_ID}"
          aws ec2 associate-iam-instance-profile \
               --instance-id "${INSTANCE_ID}" \
               --iam-instance-profile Name="${PROFILE_NAME}" --region "${REGION}" >/dev/null
     fi

     INSTANCE_PLATFORM=$(aws ec2 describe-instances \
          --instance-ids "${INSTANCE_ID}" \
          --region "${REGION}" \
          --query 'Reservations[0].Instances[0].Platform' --output text 2>/dev/null || true)

     if [ "${SKIP_INSTALL}" != "true" ]; then
          SSM_STATUS=$(aws ssm describe-instance-information \
               --filters "Key=InstanceIds,Values=${INSTANCE_ID}" \
               --query 'InstanceInformationList[0].PingStatus' \
               --region "${REGION}" --output text 2>/dev/null || true)

          if [ "${SSM_STATUS}" = "Online" ]; then
               section "Installing agent on ${INSTANCE_ID}..."
               install_env=""
               [ -n "${INSTALL_URL}" ] && install_env="CWAGENT_INSTALL_URL=${INSTALL_URL}"
               if [ "${INSTANCE_PLATFORM}" = "windows" ]; then
                    ps_prelude=""
                    [ -n "${INSTALL_URL}" ] && ps_prelude="\$env:CWAGENT_INSTALL_URL='${INSTALL_URL}'; "
                    if INSTALL_CMD=$(windows_install_cmd "${ps_prelude}"); then
                         run_via_ssm "AWS-RunPowerShellScript" "${INSTALL_CMD}"
                         log "Agent installed on ${INSTANCE_ID}"
                         return
                    fi
               else
                    if INSTALL_CMD=$(linux_install_cmd "${install_env}"); then
                         run_via_ssm "AWS-RunShellScript" "${INSTALL_CMD}"
                         log "Agent installed on ${INSTANCE_ID}"
                         return
                    fi
               fi
               logwarn "install script not found next to setup.sh and CWAGENT_INSTALL_SCRIPT_URL not set"
          else
               logwarn "SSM agent is not available on ${INSTANCE_ID}"
          fi
     fi

     echo ""
     echo "Done. Run the following on ${INSTANCE_ID} to install and start the agent:"
     echo ""
     if [ "${INSTANCE_PLATFORM}" = "windows" ]; then
          print_windows_install
          echo ""
          echo "  # Configure and start with the default OpenTelemetry config:"
          echo "  ${CTL_PS1} -Action fetch-config -Mode ec2 -ConfigLocation default:otel -Start"
     else
          print_linux_install
          echo ""
          echo "  # Configure and start with the default OpenTelemetry config:"
          echo "  sudo ${CTL} -a fetch-config -m ec2 -c default:otel -s"
     fi
}

# =============================================================================
# AWS EKS
# =============================================================================

setup_aws_eks() {
     if [ -z "${CLUSTER_NAME}" ] || [ -z "${REGION}" ]; then usage; fi

     section "Configuring EKS Pod Identity..."

     if aws eks describe-addon --cluster-name "${CLUSTER_NAME}" --addon-name eks-pod-identity-agent --region "${REGION}" >/dev/null 2>&1; then
          log "Pod Identity Agent addon installed"
     else
          logaction "Installing Pod Identity Agent addon"
          aws eks create-addon \
               --cluster-name "${CLUSTER_NAME}" \
               --addon-name eks-pod-identity-agent \
               --region "${REGION}" >/dev/null
     fi

     section "Configuring IAM role..."

     ensure_iam_role '{
    "Effect": "Allow",
    "Principal": { "Service": "pods.eks.amazonaws.com" },
    "Action": ["sts:AssumeRole", "sts:TagSession"]
  }'

     attach_permissions_policy

     ROLE_ARN=$(aws iam get-role \
          --role-name "${ROLE_NAME}" \
          --query Role.Arn --output text)

     section "Configuring pod identity association..."

     EXISTING_ASSOC=$(aws eks list-pod-identity-associations \
          --cluster-name "${CLUSTER_NAME}" \
          --namespace "${NAMESPACE}" \
          --service-account cloudwatch-agent \
          --region "${REGION}" \
          --query 'associations[0].associationId' --output text 2>/dev/null || true)

     if [ -n "${EXISTING_ASSOC}" ] && [ "${EXISTING_ASSOC}" != "None" ]; then
          EXISTING_ROLE=$(aws eks describe-pod-identity-association \
               --cluster-name "${CLUSTER_NAME}" \
               --association-id "${EXISTING_ASSOC}" \
               --region "${REGION}" \
               --query 'association.roleArn' --output text 2>/dev/null || true)
          if [ "${EXISTING_ROLE}" = "${ROLE_ARN}" ]; then
               log "Pod identity association exists"
          else
               logaction "Updating association role to ${ROLE_ARN}"
               aws eks update-pod-identity-association \
                    --cluster-name "${CLUSTER_NAME}" \
                    --association-id "${EXISTING_ASSOC}" \
                    --role-arn "${ROLE_ARN}" \
                    --region "${REGION}" >/dev/null
          fi
     else
          logaction "Creating association for ${NAMESPACE}/cloudwatch-agent"
          aws eks create-pod-identity-association \
               --cluster-name "${CLUSTER_NAME}" \
               --region "${REGION}" \
               --namespace "${NAMESPACE}" \
               --service-account cloudwatch-agent \
               --role-arn "${ROLE_ARN}" >/dev/null
     fi

     EKS_ADDON_CONFIG='{"agent":{"env":[{"name":"USE_DEFAULT_CONFIG","value":"otel"}]}}'

     if [ "${SKIP_INSTALL}" != "true" ]; then
          section "Installing ${EKS_ADDON_NAME} add-on..."
          if aws eks describe-addon --cluster-name "${CLUSTER_NAME}" --addon-name "${EKS_ADDON_NAME}" --region "${REGION}" >/dev/null 2>&1; then
               logaction "Updating existing add-on"
               aws eks update-addon \
                    --cluster-name "${CLUSTER_NAME}" \
                    --addon-name "${EKS_ADDON_NAME}" \
                    --configuration-values "${EKS_ADDON_CONFIG}" \
                    --region "${REGION}" >/dev/null
          else
               logaction "Creating add-on"
               aws eks create-addon \
                    --cluster-name "${CLUSTER_NAME}" \
                    --addon-name "${EKS_ADDON_NAME}" \
                    --configuration-values "${EKS_ADDON_CONFIG}" \
                    --region "${REGION}" >/dev/null
          fi
          log "Add-on ${EKS_ADDON_NAME} installed on ${CLUSTER_NAME}"
     else
          echo ""
          echo "Done. Install the Amazon CloudWatch Observability EKS add-on:"
          echo ""
          printf '  aws eks create-addon \\\n'
          printf '    --cluster-name %s \\\n' "${CLUSTER_NAME}"
          printf '    --addon-name %s \\\n' "${EKS_ADDON_NAME}"
          printf '    --configuration-values %s \\\n' "'${EKS_ADDON_CONFIG}'"
          printf '    --region %s\n' "${REGION}"
     fi
}

# =============================================================================
# AWS ECS
# =============================================================================

setup_aws_ecs() {
     section "Configuring IAM task role..."

     ensure_iam_role '{
    "Effect": "Allow",
    "Principal": { "Service": "ecs-tasks.amazonaws.com" },
    "Action": "sts:AssumeRole"
  }'

     attach_permissions_policy

     ROLE_ARN=$(aws iam get-role \
          --role-name "${ROLE_NAME}" \
          --query Role.Arn --output text)

     section "Add this container to your task definition's containerDefinitions:"
     echo ""
     cat <<EOF
    {
      "name": "cloudwatch-agent",
      "image": "${CONTAINER_IMAGE}",
      "essential": false,$(
          if [ "${ECS_LAUNCH_TYPE}" = "ec2" ]; then
               cat <<PORTS

      "portMappings": [
        { "containerPort": 4317, "hostPort": 4317, "protocol": "tcp" },
        { "containerPort": 4318, "hostPort": 4318, "protocol": "tcp" }
      ],
PORTS
          fi
     )
      "environment": [
        { "name": "USE_DEFAULT_CONFIG", "value": "otel" }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-create-group": "True",
          "awslogs-group": "/ecs/cloudwatch-agent",
          "awslogs-region": "${REGION}",
          "awslogs-stream-prefix": "agent"
        }
      }
    }
EOF

     echo ""
     echo "Also set on your task definition:"
     if [ "${ECS_LAUNCH_TYPE}" = "ec2" ]; then
          echo "  \"taskRoleArn\": \"${ROLE_ARN}\","
          echo "  \"networkMode\": \"bridge\""
          echo ""
          echo "Add to your app container:"
          echo "  \"links\": [\"cloudwatch-agent\"]"
          echo "  \"environment\": [{\"name\": \"OTEL_EXPORTER_OTLP_ENDPOINT\", \"value\": \"http://cloudwatch-agent:4317\"}]"
     else
          echo "  \"taskRoleArn\": \"${ROLE_ARN}\""
     fi
}

# =============================================================================
# Azure VM
# =============================================================================

setup_azure_vm() {
     if [ -z "${RESOURCE_GROUP}" ] || [ -z "${VM_NAME}" ]; then usage; fi

     OIDC_AUDIENCE="https://management.azure.com/"

     section "Configuring Azure VM identity..."

     IDENTITY=$(az vm show \
          --resource-group "${RESOURCE_GROUP}" \
          --name "${VM_NAME}" \
          --query "identity.principalId" -o tsv 2>/dev/null || true)

     if [ -n "${IDENTITY}" ] && [ "${IDENTITY}" != "None" ]; then
          log "Managed identity enabled on ${VM_NAME}"
     else
          logaction "Enabling managed identity (this may take a few minutes)"
          az vm identity assign \
               --resource-group "${RESOURCE_GROUP}" \
               --name "${VM_NAME}" \
               --output none
          IDENTITY=$(az vm show --resource-group "${RESOURCE_GROUP}" --name "${VM_NAME}" \
               --query identity.principalId -o tsv)
     fi

     # Assign Reader on subscription for cloud.account.name resolution
     SUB_ID=$(az account show --query id -o tsv)
     if ! az role assignment list --assignee "${IDENTITY}" --role Reader \
          --scope "/subscriptions/${SUB_ID}" --query '[0].id' -o tsv 2>/dev/null | grep -q .; then
          az role assignment create \
               --assignee-object-id "${IDENTITY}" \
               --assignee-principal-type ServicePrincipal \
               --role Reader \
               --scope "/subscriptions/${SUB_ID}" \
               --output none
          log "Reader role assigned to VM identity"
     else
          log "Reader role already assigned to VM identity"
     fi

     section "Configuring AWS trust..."

     TENANT_ID=$(az account show --query tenantId -o tsv)

     PROVIDER_ARN=$(aws iam list-open-id-connect-providers \
          --query "OpenIDConnectProviderList[?ends_with(Arn, 'sts.windows.net/${TENANT_ID}/')].Arn | [0]" \
          --output text 2>/dev/null || true)

     if [ -n "${PROVIDER_ARN}" ] && [ "${PROVIDER_ARN}" != "None" ]; then
          log "OIDC provider exists"
     else
          logaction "Registering OIDC provider"
          aws iam create-open-id-connect-provider \
               --url "https://sts.windows.net/${TENANT_ID}/" \
               --client-id-list "${OIDC_AUDIENCE}" \
               --thumbprint-list "626d44e704d1ceabe3bf0d53397464ac8080142c" \
               >/dev/null
     fi

     ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

     TRUST_STATEMENT=$(
          cat <<EOF
{
    "Effect": "Allow",
    "Principal": {
      "Federated": "arn:aws:iam::${ACCOUNT_ID}:oidc-provider/sts.windows.net/${TENANT_ID}/"
    },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": {
        "sts.windows.net/${TENANT_ID}/:aud": "${OIDC_AUDIENCE}"
      }
    }
  }
EOF
     )

     ensure_iam_role "${TRUST_STATEMENT}"

     attach_permissions_policy

     ROLE_ARN=$(aws iam get-role \
          --role-name "${ROLE_NAME}" \
          --query Role.Arn --output text)

     VM_OS=$(az vm show \
          --resource-group "${RESOURCE_GROUP}" \
          --name "${VM_NAME}" \
          --query "storageProfile.osDisk.osType" -o tsv 2>/dev/null || true)

     if [ "${SKIP_INSTALL}" != "true" ]; then
          section "Installing agent on ${VM_NAME}..."
          if [ "${VM_OS}" = "Windows" ]; then
               ps_prelude="\$env:CWAGENT_CLOUD='azure'; \$env:CWAGENT_ROLE_ARN='${ROLE_ARN}'; \$env:CWAGENT_AWS_REGION='${REGION}'; "
               [ -n "${INSTALL_URL}" ] && ps_prelude="${ps_prelude}\$env:CWAGENT_INSTALL_URL='${INSTALL_URL}'; "
               if INSTALL_CMD=$(windows_install_cmd "${ps_prelude}"); then
                    run_via_az "RunPowerShellScript" "${INSTALL_CMD}"
                    log "Agent installed on ${VM_NAME}"
                    return
               fi
          else
               install_env="CWAGENT_CLOUD=azure CWAGENT_ROLE_ARN=${ROLE_ARN} CWAGENT_AWS_REGION=${REGION}"
               [ -n "${INSTALL_URL}" ] && install_env="${install_env} CWAGENT_INSTALL_URL=${INSTALL_URL}"
               if INSTALL_CMD=$(linux_install_cmd "${install_env}"); then
                    run_via_az "RunShellScript" "${INSTALL_CMD}"
                    log "Agent installed on ${VM_NAME}"
                    return
               fi
          fi
          logwarn "install script not found next to setup.sh and CWAGENT_INSTALL_SCRIPT_URL not set"
     fi

     echo ""
     echo "Done. Run the following on ${VM_NAME} to install and start the agent:"
     echo ""
     if [ "${VM_OS}" = "Windows" ]; then
          print_windows_install
          echo ""
          echo "  # Configure credentials and region, then start with the default OpenTelemetry config:"
          echo "  ${CTL_PS1} -Action set-env -EnvVar CWAGENT_ROLE_ARN=${ROLE_ARN}"
          echo "  ${CTL_PS1} -Action set-env -EnvVar AWS_REGION=${REGION}"
          echo "  ${CTL_PS1} -Action fetch-config -Mode onPremise -ConfigLocation default:otel -Start"
     else
          print_linux_install
          echo ""
          echo "  # Configure credentials and region, then start with the default OpenTelemetry config:"
          echo "  sudo ${CTL} -a set-env -e CWAGENT_ROLE_ARN=${ROLE_ARN}"
          echo "  sudo ${CTL} -a set-env -e AWS_REGION=${REGION}"
          echo "  sudo ${CTL} -a fetch-config -m onPremise -c default:otel -s"
     fi
}

# =============================================================================
# Azure AKS
# =============================================================================

setup_azure_aks() {
     if [ -z "${RESOURCE_GROUP}" ] || [ -z "${CLUSTER_NAME}" ]; then usage; fi

     section "Configuring AKS cluster..."

     OIDC_ENABLED=$(az aks show \
          --resource-group "${RESOURCE_GROUP}" \
          --name "${CLUSTER_NAME}" \
          --query "oidcIssuerProfile.enabled" -o tsv 2>/dev/null || true)

     if [ "${OIDC_ENABLED}" = "true" ]; then
          log "OIDC issuer and workload identity enabled"
     else
          logaction "Enabling OIDC issuer and workload identity (this may take a few minutes)"
          az aks update \
               --resource-group "${RESOURCE_GROUP}" \
               --name "${CLUSTER_NAME}" \
               --enable-oidc-issuer \
               --enable-workload-identity \
               --output none
     fi

     OIDC_ISSUER=$(az aks show \
          --resource-group "${RESOURCE_GROUP}" \
          --name "${CLUSTER_NAME}" \
          --query "oidcIssuerProfile.issuerUrl" -o tsv)

     OIDC_HOST="${OIDC_ISSUER#https://}"

     # Assign Reader on subscription for cloud.account.name resolution
     KUBELET_IDENTITY=$(az aks show --resource-group "${RESOURCE_GROUP}" --name "${CLUSTER_NAME}" \
          --query "identityProfile.kubeletidentity.objectId" -o tsv)
     SUB_ID=$(az account show --query id -o tsv)
     if [ -n "${KUBELET_IDENTITY}" ]; then
          if ! az role assignment list --assignee "${KUBELET_IDENTITY}" --role Reader \
               --scope "/subscriptions/${SUB_ID}" --query '[0].id' -o tsv 2>/dev/null | grep -q .; then
               az role assignment create \
                    --assignee-object-id "${KUBELET_IDENTITY}" \
                    --assignee-principal-type ServicePrincipal \
                    --role Reader \
                    --scope "/subscriptions/${SUB_ID}" \
                    --output none
               log "Reader role assigned to kubelet identity"
          else
               log "Reader role already assigned to kubelet identity"
          fi
     fi

     section "Configuring AWS trust..."

     PROVIDER_ARN=$(aws iam list-open-id-connect-providers \
          --query "OpenIDConnectProviderList[?contains(Arn, '${OIDC_HOST}')].Arn | [0]" \
          --output text 2>/dev/null || true)

     if [ -n "${PROVIDER_ARN}" ] && [ "${PROVIDER_ARN}" != "None" ]; then
          log "OIDC provider exists"
     else
          logaction "Registering OIDC provider"
          aws iam create-open-id-connect-provider \
               --url "${OIDC_ISSUER}" \
               --client-id-list sts.amazonaws.com \
               >/dev/null
     fi

     ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

     TRUST_STATEMENT=$(
          cat <<EOF
{
    "Effect": "Allow",
    "Principal": {
      "Federated": "arn:aws:iam::${ACCOUNT_ID}:oidc-provider/${OIDC_HOST}"
    },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": {
        "${OIDC_HOST}:sub": "system:serviceaccount:${NAMESPACE}:cloudwatch-agent",
        "${OIDC_HOST}:aud": "sts.amazonaws.com"
      }
    }
  }
EOF
     )

     ensure_iam_role "${TRUST_STATEMENT}"

     attach_permissions_policy

     ROLE_ARN=$(aws iam get-role \
          --role-name "${ROLE_NAME}" \
          --query Role.Arn --output text)

     if [ "${SKIP_INSTALL}" != "true" ]; then
          section "Installing CloudWatch Observability Helm chart on ${CLUSTER_NAME}..."
          logaction "Configuring kubeconfig for ${CLUSTER_NAME}"
          az aks get-credentials --resource-group "${RESOURCE_GROUP}" --name "${CLUSTER_NAME}" --overwrite-existing

          logaction "Installing via Helm"
          helm repo add aws-observability "${HELM_CHART_REPO}" 2>/dev/null || true
          helm repo update aws-observability
          helm upgrade --install amazon-cloudwatch-observability aws-observability/amazon-cloudwatch-observability \
               --set k8sMode=AKS \
               --set roleArn="${ROLE_ARN}" \
               --set region="${REGION}" \
               --set clusterName="${CLUSTER_NAME}" \
               --set-json 'agent.env=[{"name":"USE_DEFAULT_CONFIG","value":"otel"}]' \
               --namespace "${NAMESPACE}" \
               --create-namespace
          log "Chart installed on ${CLUSTER_NAME}"
     else
          echo ""
          echo "Done. Install the Amazon CloudWatch Observability Helm chart (requires kubeconfig for ${CLUSTER_NAME}):"
          echo ""
          echo "  helm repo add aws-observability ${HELM_CHART_REPO}"
          echo "  helm repo update aws-observability"
          printf '  helm upgrade --install amazon-cloudwatch-observability aws-observability/amazon-cloudwatch-observability \\\n'
          printf '    --set k8sMode=AKS \\\n'
          printf '    --set roleArn=%s \\\n' "${ROLE_ARN}"
          printf '    --set region=%s \\\n' "${REGION}"
          printf '    --set clusterName=%s \\\n' "${CLUSTER_NAME}"
          printf '    --set-json %s \\\n' "'agent.env=[{\"name\":\"USE_DEFAULT_CONFIG\",\"value\":\"otel\"}]'"
          printf '    --namespace %s \\\n' "${NAMESPACE}"
          printf '    --create-namespace\n'
     fi
}

# =============================================================================
# Input validation
# =============================================================================

validate_inputs() {
     case "${PLATFORM}" in
     aws_ec2)
          echo "${INSTANCE_ID}" | grep -qE '^i-[0-9a-f]{8}([0-9a-f]{9})?$' || die "invalid instance ID: ${INSTANCE_ID}"
          ;;
     aws_eks)
          if [ -z "${CLUSTER_NAME}" ]; then die "cluster name is required"; fi
          echo "${REGION}" | grep -qE '^[a-z]+-[a-z]+-[0-9]+$' || die "invalid region: ${REGION}"
          ;;
     azure_vm)
          if [ -z "${RESOURCE_GROUP}" ]; then die "resource group is required"; fi
          if [ -z "${VM_NAME}" ]; then die "VM name is required"; fi
          ;;
     azure_aks)
          if [ -z "${RESOURCE_GROUP}" ]; then die "resource group is required"; fi
          if [ -z "${CLUSTER_NAME}" ]; then die "cluster name is required"; fi
          ;;
     esac
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
          read -r input
          input="${input:-${default}}"
          [ -n "${input}" ] && break
     done
     eval "${var}=\"${input}\""
}

interactive_setup() {
     printf '\nSelect platform:\n'
     printf '  aws_ec2     EC2 instance\n'
     printf '  aws_ecs     ECS task (sidecar)\n'
     printf '  aws_eks     EKS cluster (add-on)\n'
     printf '  azure_vm    Azure VM\n'
     printf '  azure_aks   AKS cluster (Helm)\n'
     ask "Platform:"
     read -r choice
     case "${choice}" in
     aws_ec2) PLATFORM=aws_ec2 ;;
     aws_ecs) PLATFORM=aws_ecs ;;
     aws_eks) PLATFORM=aws_eks ;;
     azure_vm) PLATFORM=azure_vm ;;
     azure_aks) PLATFORM=azure_aks ;;
     *) die "invalid platform: ${choice}" ;;
     esac

     printf '\n'
     case "${PLATFORM}" in
     aws_ec2)
          prompt INSTANCE_ID "Instance ID"
          # Discover existing profile/role as defaults
          CURRENT_PROFILE_ARN=$(aws ec2 describe-iam-instance-profile-associations \
               --filters "Name=instance-id,Values=${INSTANCE_ID}" "Name=state,Values=associated" \
               --query 'IamInstanceProfileAssociations[0].IamInstanceProfile.Arn' \
               --region "${REGION:-${DETECTED_REGION:-us-east-1}}" --output text 2>/dev/null || true)
          if [ -n "${CURRENT_PROFILE_ARN}" ] && [ "${CURRENT_PROFILE_ARN}" != "None" ]; then
               DISCOVERED_PROFILE="${CURRENT_PROFILE_ARN##*/}"
               DISCOVERED_ROLE=$(aws iam get-instance-profile \
                    --instance-profile-name "${DISCOVERED_PROFILE}" \
                    --query 'InstanceProfile.Roles[0].RoleName' --output text 2>/dev/null || true)
               if [ -n "${DISCOVERED_ROLE}" ] && [ "${DISCOVERED_ROLE}" != "None" ]; then
                    log "Found instance profile: ${DISCOVERED_PROFILE} (role: ${DISCOVERED_ROLE})"
                    DEFAULT_ROLE="${DISCOVERED_ROLE}"
                    ROLE_NAME=""
               fi
          fi
          prompt ROLE_NAME "IAM role name" "${DEFAULT_ROLE:-CloudWatchAgentServerRole}"
          ;;
     aws_ecs)
          prompt ECS_LAUNCH_TYPE "Launch type (fargate|ec2)" "fargate"
          ;;
     aws_eks)
          prompt CLUSTER_NAME "Cluster name"
          prompt REGION "Region" "${DETECTED_REGION}"
          prompt NAMESPACE "Namespace" "amazon-cloudwatch"
          ;;
     azure_vm)
          prompt RESOURCE_GROUP "Resource group"
          prompt VM_NAME "VM name"
          ;;
     azure_aks)
          prompt RESOURCE_GROUP "Resource group"
          prompt CLUSTER_NAME "Cluster name"
          prompt NAMESPACE "Namespace" "amazon-cloudwatch"
          ;;
     esac

     prompt ROLE_NAME "IAM role name" "${ROLE_NAME}"

     validate_inputs

     printf '\n  Platform:    %s\n' "${PLATFORM}"
     case "${PLATFORM}" in
     aws_ec2)
          printf '  Instance:    %s\n' "${INSTANCE_ID}"
          ;;
     aws_ecs) printf '  Launch type: %s\n' "${ECS_LAUNCH_TYPE}" ;;
     aws_eks) printf '  Cluster:     %s\n  Region:      %s\n  Namespace:   %s\n' "${CLUSTER_NAME}" "${REGION}" "${NAMESPACE}" ;;
     azure_vm) printf '  RG:          %s\n  VM:          %s\n' "${RESOURCE_GROUP}" "${VM_NAME}" ;;
     azure_aks) printf '  RG:          %s\n  Cluster:     %s\n  Namespace:   %s\n' "${RESOURCE_GROUP}" "${CLUSTER_NAME}" "${NAMESPACE}" ;;
     esac
     printf '  Role:        %s\n' "${ROLE_NAME}"

     printf '\n'
     ask "Proceed? [Y/n]"
     read -r answer
     case "${answer}" in [nN]*)
          echo "Aborted."
          exit 0
          ;;
     esac
}

# =============================================================================
# Main
# =============================================================================

main() {
     case "${1:-}" in -h | --help) usage ;; esac

     if [ -t 0 ] && [ -z "${PLATFORM}" ]; then
          interactive_setup
     else
          if [ -z "${PLATFORM}" ]; then usage; fi
     fi

     case "${PLATFORM}" in
     aws_ec2 | aws_ecs | aws_eks | azure_vm | azure_aks) ;;
     *)
          die "unsupported platform: ${PLATFORM} (valid: aws_ec2, aws_ecs, aws_eks, azure_vm, azure_aks)"
          ;;
     esac

     check_prerequisites

     # Detect configured region now that AWS CLI is confirmed
     if [ -z "${DETECTED_REGION}" ]; then
          DETECTED_REGION=$(aws configure get region 2>/dev/null || true)
     fi
     REGION="${REGION:-${DETECTED_REGION:-us-east-1}}"

     # If TTY and required args are missing, prompt for them
     if [ -t 0 ]; then
          case "${PLATFORM}" in
          aws_ec2) prompt INSTANCE_ID "Instance ID" ;;
          aws_eks)
               prompt CLUSTER_NAME "Cluster name"
               prompt REGION "Region" "${DETECTED_REGION}"
               ;;
          azure_vm)
               prompt RESOURCE_GROUP "Resource group"
               prompt VM_NAME "VM name"
               ;;
          azure_aks)
               prompt RESOURCE_GROUP "Resource group"
               prompt CLUSTER_NAME "Cluster name"
               ;;
          esac
     fi

     validate_inputs

     ensure_transaction_search

     # Apply defaults for non-interactive usage
     NAMESPACE="${NAMESPACE:-amazon-cloudwatch}"
     ECS_LAUNCH_TYPE="${ECS_LAUNCH_TYPE:-fargate}"

     case "${PLATFORM}" in
     aws_ec2) setup_aws_ec2 ;;
     aws_ecs) setup_aws_ecs ;;
     aws_eks) setup_aws_eks ;;
     azure_vm) setup_azure_vm ;;
     azure_aks) setup_azure_aks ;;
     esac
}

main "$@"
