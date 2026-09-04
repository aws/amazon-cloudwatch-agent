#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# Amazon CloudWatch Agent - AWS setup (trust + install)
#
# Sets up the IAM role the agent assumes to send OpenTelemetry (OTLP) telemetry
# to CloudWatch and attaches CloudWatchAgentServerPolicy. What happens after that
# depends on the platform:
#
#   aws_ec2     trust (instance-profile role) + install via SSM
#   aws_ecs     trust (task role) + print the sidecar container definition
#   aws_eks     trust (Pod Identity) + install the Observability EKS add-on
#   azure_vm    trust only (web-identity for the Azure tenant OIDC provider),
#               install runs on the Azure side via azure/setup.sh
#   azure_aks   trust only (web-identity for the AKS issuer OIDC provider),
#               install runs on the Azure side via azure/setup.sh
#   gcp_vm      trust only (web-identity federated with accounts.google.com),
#               install runs on the GCP side via gcp/setup.sh
#   gcp_gke     trust only (web-identity for the GKE issuer OIDC provider),
#               install runs on the GCP side via gcp/setup.sh
#
# Safe to re-run: trust statements and policies are merged, not replaced, and an
# instance keeps its profile.
#
# Requires IAM write access, "aws", and "jq". For azure_vm and azure_aks it also
# takes an identity value from the Azure setup (the tenant ID or the OIDC issuer
# URL); gcp_vm and gcp_gke take one from the GCP setup (the service account
# unique ID or the cluster OIDC issuer URL). Outputs the role ARN.
#
# Usage:
#     CWAGENT_PLATFORM=aws_eks \
#     CWAGENT_K8S_CLUSTER_NAME=my-cluster \
#     CWAGENT_AWS_REGION=us-east-1 \
#       ./aws/setup.sh
#
# Environment variables:
#   Common:
#     CWAGENT_PLATFORM                        aws_ec2 | aws_ecs | aws_eks | azure_vm | azure_aks | gcp_vm | gcp_gke
#     CWAGENT_AWS_ROLE_NAME                   IAM role name (default: CloudWatchAgentServerRole)
#     CWAGENT_AWS_REGION                      AWS region telemetry is sent to (required,
#                                             falls back to the AWS CLI config if unset)
#     CWAGENT_EMIT_ENV                        When set (1/true/yes/on), print eval-able
#                                             KEY='value' lines on stdout, logging to stderr
#     CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH   When set (1/true/yes/on), enable Transaction
#                                             Search (a per-region account setting OTLP
#                                             traces need) if it is off
#   aws_ec2:
#     CWAGENT_AWS_INSTANCE_ID                 EC2 instance ID
#     CWAGENT_AWS_UPDATE_INSTANCE_ROLE        When set (1/true/yes/on), attach the policy
#                                             to the role already on the instance
#   aws_ecs:
#     CWAGENT_AWS_ECS_LAUNCH_TYPE             fargate | ec2 (default: fargate)
#   aws_eks:
#     CWAGENT_K8S_CLUSTER_NAME                Cluster name
#   azure_vm:
#     CWAGENT_AZURE_TENANT_ID                 Azure tenant ID
#   azure_aks:
#     CWAGENT_AZURE_OIDC_ISSUER               AKS OIDC issuer URL
#   gcp_vm:
#     CWAGENT_GCP_SA_UNIQUE_ID                GCP service account unique ID
#   gcp_gke:
#     CWAGENT_GCP_OIDC_ISSUER                 GKE cluster OIDC issuer URL

set -eu

PLATFORM="${CWAGENT_PLATFORM:-}"
TENANT_ID="${CWAGENT_AZURE_TENANT_ID:-}"
OIDC_ISSUER="${CWAGENT_AZURE_OIDC_ISSUER:-}"
SA_UNIQUE_ID="${CWAGENT_GCP_SA_UNIQUE_ID:-}"
GCP_OIDC_ISSUER="${CWAGENT_GCP_OIDC_ISSUER:-}"
INSTANCE_ID="${CWAGENT_AWS_INSTANCE_ID:-}"
UPDATE_INSTANCE_ROLE="${CWAGENT_AWS_UPDATE_INSTANCE_ROLE:-}"
ENABLE_TXN_SEARCH="${CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH:-}"
CLUSTER_NAME="${CWAGENT_K8S_CLUSTER_NAME:-}"
ROLE_NAME="${CWAGENT_AWS_ROLE_NAME:-CloudWatchAgentServerRole}"
REGION="${CWAGENT_AWS_REGION:-}"
EMIT_ENV="${CWAGENT_EMIT_ENV:-}"
ECS_LAUNCH_TYPE="${CWAGENT_AWS_ECS_LAUNCH_TYPE:-}"

# Where the target fetches the install payload (install.sh / install.ps1) from.
SCRIPT_BASE_URL="https://raw.githubusercontent.com/aws/amazon-cloudwatch-agent/main/scripts"
# The agent's namespace is fixed: the EKS add-on always installs into
# amazon-cloudwatch, and the operator/chart centers on it, so it is not settable.
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
# parent shell captures and evaluates, so readable output (including any install
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
  CWAGENT_PLATFORM=aws_eks   CWAGENT_AWS_REGION=us-east-1 CWAGENT_K8S_CLUSTER_NAME=<cluster>    $0
  CWAGENT_PLATFORM=aws_ec2   CWAGENT_AWS_REGION=us-east-1 CWAGENT_AWS_INSTANCE_ID=i-123         $0
  CWAGENT_PLATFORM=aws_ecs   CWAGENT_AWS_REGION=us-east-1                                       $0
  CWAGENT_PLATFORM=azure_aks CWAGENT_AWS_REGION=us-east-1 CWAGENT_AZURE_OIDC_ISSUER=https://... $0
  CWAGENT_PLATFORM=azure_vm  CWAGENT_AWS_REGION=us-east-1 CWAGENT_AZURE_TENANT_ID=<tenant>      $0
  CWAGENT_PLATFORM=gcp_vm    CWAGENT_AWS_REGION=us-east-1 CWAGENT_GCP_SA_UNIQUE_ID=<unique-id>  $0
  CWAGENT_PLATFORM=gcp_gke   CWAGENT_AWS_REGION=us-east-1 CWAGENT_GCP_OIDC_ISSUER=https://...   $0

Environment variables:
  Common:
    CWAGENT_PLATFORM                        aws_ec2 | aws_ecs | aws_eks | azure_vm | azure_aks | gcp_vm | gcp_gke
    CWAGENT_AWS_ROLE_NAME                   IAM role name (default: CloudWatchAgentServerRole)
    CWAGENT_AWS_REGION                      AWS region telemetry is sent to (required)
    CWAGENT_EMIT_ENV                        Print eval-able KEY='value' lines on stdout
    CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH   Enable Transaction Search in the region
  aws_ec2:
    CWAGENT_AWS_INSTANCE_ID                 EC2 instance ID
    CWAGENT_AWS_UPDATE_INSTANCE_ROLE        Attach the policy to the existing role
  aws_ecs:
    CWAGENT_AWS_ECS_LAUNCH_TYPE             fargate | ec2 (default: fargate)
  aws_eks:
    CWAGENT_K8S_CLUSTER_NAME                Cluster name
  azure_vm:
    CWAGENT_AZURE_TENANT_ID                 Azure tenant ID
  azure_aks:
    CWAGENT_AZURE_OIDC_ISSUER               AKS OIDC issuer URL
  gcp_vm:
    CWAGENT_GCP_SA_UNIQUE_ID                GCP service account unique ID
  gcp_gke:
    CWAGENT_GCP_OIDC_ISSUER                 GKE cluster OIDC issuer URL
EOF
     exit "${rc}"
}

# =============================================================================
# Env-var emit (CWAGENT_EMIT_ENV)
#
# Under CWAGENT_EMIT_ENV, emit_env prints accumulated values as eval-able
# KEY='value' lines on stdout. Single-quoted for the documented
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

# =============================================================================
# Interactive mode
#
# The identity values (tenant ID, OIDC issuer, service account unique ID)
# come from the Azure or GCP identity setup. When run by hand they can be
# pasted in at the prompts rather than passed through the environment.
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
     printf '  aws_ec2     EC2 instance\n' >&3
     printf '  aws_ecs     ECS task (sidecar)\n' >&3
     printf '  aws_eks     EKS cluster\n' >&3
     printf '  azure_vm    Azure VM\n' >&3
     printf '  azure_aks   AKS cluster\n' >&3
     printf '  gcp_vm      GCE VM\n' >&3
     printf '  gcp_gke     GKE cluster\n' >&3
     ask "Platform:"
     read -r choice || die "no platform selected"
     case "${choice}" in
     aws_ec2) PLATFORM=aws_ec2 ;;
     aws_ecs) PLATFORM=aws_ecs ;;
     aws_eks) PLATFORM=aws_eks ;;
     azure_vm) PLATFORM=azure_vm ;;
     azure_aks) PLATFORM=azure_aks ;;
     gcp_vm) PLATFORM=gcp_vm ;;
     gcp_gke) PLATFORM=gcp_gke ;;
     *) die "invalid platform: ${choice}" ;;
     esac

     printf '\n' >&3
     case "${PLATFORM}" in
     aws_ec2)
          prompt INSTANCE_ID "Instance ID"
          ;;
     aws_ecs)
          prompt ECS_LAUNCH_TYPE "Launch type (fargate|ec2)" "fargate"
          ;;
     aws_eks)
          prompt CLUSTER_NAME "Cluster name"
          ;;
     azure_vm)
          prompt TENANT_ID "Azure tenant ID"
          ;;
     azure_aks)
          prompt OIDC_ISSUER "AKS OIDC issuer URL"
          ;;
     gcp_vm)
          prompt SA_UNIQUE_ID "GCP service account unique ID"
          ;;
     gcp_gke)
          prompt GCP_OIDC_ISSUER "GKE cluster OIDC issuer URL"
          ;;
     esac
     # `prompt` returns early on a non-empty current value, and ROLE_NAME always
     # has one (the built-in default), so ask directly with the default filled in.
     ask "IAM role name [${ROLE_NAME}]:"
     read -r role_input || die "no input for IAM role name"
     [ -n "${role_input}" ] && ROLE_NAME="${role_input}"
}

check_prerequisites() {
     command -v aws >/dev/null 2>&1 || die "AWS CLI is required but not installed"
     command -v jq >/dev/null 2>&1 || die "jq is required but not installed"
     AWS_CLI_VERSION=$(aws --version 2>&1 | sed -n 's|.*aws-cli/\([0-9.]*\).*|\1|p' | head -n 1)
     [ -n "${AWS_CLI_VERSION}" ] || AWS_CLI_VERSION="0.0.0"
     if ! version_ge "${AWS_CLI_VERSION}" "2.22.0"; then
          logwarn "AWS CLI ${AWS_CLI_VERSION} detected (2.22+ recommended for full functionality)"
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
}

# =============================================================================
# Shared helpers
# =============================================================================

ensure_iam_role() {
     new_statement="$1"
     full_policy="{\"Version\":\"2012-10-17\",\"Statement\":[${new_statement}]}"

     # One get-role serves both the existence probe and the current-policy fetch:
     # a failure means the role is absent, so create it and return.
     if ! existing=$(aws iam get-role --role-name "${ROLE_NAME}" \
          --query 'Role.AssumeRolePolicyDocument' --output json 2>/dev/null); then
          logaction "Creating IAM role ${ROLE_NAME}"
          aws iam create-role \
               --role-name "${ROLE_NAME}" \
               --assume-role-policy-document "${full_policy}" \
               >/dev/null
          return
     fi

     new_principal=$(printf '%s' "${new_statement}" | jq -r \
          '(.Principal | if type == "object" then to_entries[0].value else . end)')

     # Statements for different principals coexist on one role (EC2, ECS, EKS,
     # and the Azure federations all key off distinct principals). For this
     # principal: identical means nothing to do, different means replace (not
     # leave stale, e.g. a changed :sub namespace), absent means append.
     state=$(printf '%s' "${existing}" | jq -r \
          --arg principal "${new_principal}" \
          --argjson stmt "${new_statement}" \
          '[.Statement[] | select((.Principal | if type == "object" then to_entries[0].value else . end) == $principal)] as $m
           | if ($m | length) == 0 then "absent"
             elif ($m | any(. == $stmt)) then "current"
             else "stale" end')

     if [ "${state}" = "current" ]; then
          log "IAM role ${ROLE_NAME} trust policy up to date"
          return
     fi

     if [ "${state}" = "stale" ]; then
          logaction "Updating trust statement on ${ROLE_NAME}"
          merged=$(printf '%s' "${existing}" | jq \
               --arg principal "${new_principal}" \
               --argjson stmt "${new_statement}" \
               '.Statement = ([.Statement[] | select((.Principal | if type == "object" then to_entries[0].value else . end) != $principal)] + [$stmt])')
     else
          logaction "Merging trust statement into ${ROLE_NAME}"
          merged=$(printf '%s' "${existing}" | jq \
               --argjson stmt "${new_statement}" \
               '.Statement += [$stmt]')
     fi

     aws iam update-assume-role-policy \
          --role-name "${ROLE_NAME}" \
          --policy-document "${merged}" \
          >/dev/null
}

attach_permissions_policy() {
     # attach-role-policy is idempotent (re-attaching the same policy succeeds),
     # so let a real failure (AccessDenied, NoSuchEntity) surface via set -e
     # rather than swallowing it and falsely logging success.
     aws iam attach-role-policy \
          --role-name "${ROLE_NAME}" \
          --policy-arn arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy >/dev/null
     log "Managed policy CloudWatchAgentServerPolicy attached"
}

# Register the OIDC provider for a web-identity federation if AWS does not
# already have it. $1 = substring identifying an existing provider ARN, $2 =
# provider URL, $3 = client-id (audience), $4 = thumbprint (optional: Azure AD's
# is fixed, AKS issuers are fetched from the HTTPS endpoint so none is passed).
ensure_oidc_provider() {
     match="$1"
     url="$2"
     client_id="$3"
     thumbprint="${4:-}"

     existing=$(aws iam list-open-id-connect-providers \
          --query "OpenIDConnectProviderList[?contains(Arn, '${match}')].Arn | [0]" \
          --output text 2>/dev/null || true)
     if [ -n "${existing}" ] && [ "${existing}" != "None" ]; then
          log "OIDC provider exists"
          return
     fi

     logaction "Registering OIDC provider"
     if [ -n "${thumbprint}" ]; then
          aws iam create-open-id-connect-provider --url "${url}" \
               --client-id-list "${client_id}" --thumbprint-list "${thumbprint}" >/dev/null
     else
          aws iam create-open-id-connect-provider --url "${url}" \
               --client-id-list "${client_id}" >/dev/null
     fi
}

TXN_SEARCH_DOC="https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Enable-TransactionSearch.html"

ensure_transaction_search() {
     TRACE_DEST_RC=""
     TRACE_DEST=$(aws xray get-trace-segment-destination --region "${REGION}" --query 'Destination' --output text 2>/dev/null) || TRACE_DEST_RC=$?

     if [ -n "${TRACE_DEST_RC}" ]; then
          logwarn "Could not check Transaction Search. OTLP traces need it enabled in ${REGION}:"
          logwarn "${TXN_SEARCH_DOC}"
          return
     fi
     if [ "${TRACE_DEST}" = "CloudWatchLogs" ]; then
          return
     fi

     if [ -t 0 ] && ! is_true "${EMIT_ENV}" && ! is_true "${ENABLE_TXN_SEARCH}"; then
          ask "Enable Transaction Search for the whole account in ${REGION}? [y/N]"
          read -r answer || answer=""
          case "${answer}" in [yY]*) ENABLE_TXN_SEARCH="true" ;; esac
     fi

     if ! is_true "${ENABLE_TXN_SEARCH}"; then
          logwarn "OTLP traces need Transaction Search, which is off in ${REGION}. Enabling it"
          logwarn "changes how X-Ray traces are ingested for the whole account in this region."
          logwarn "Rerun with CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH=true to enable it, or:"
          logwarn "${TXN_SEARCH_DOC}"
          return
     fi

     logaction "Enabling Transaction Search in ${REGION}"
     # Two steps: a resource policy letting X-Ray write spans into CloudWatch
     # Logs, then flipping the trace segment destination. Without the policy the
     # destination flips but X-Ray can't write, so spans never land.
     TXN_POLICY=$(
          cat <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Sid": "TransactionSearchXRayAccess",
        "Effect": "Allow",
        "Principal": { "Service": "xray.amazonaws.com" },
        "Action": "logs:PutLogEvents",
        "Resource": [
          "arn:aws:logs:${REGION}:${AWS_ACCOUNT}:log-group:aws/spans:*",
          "arn:aws:logs:${REGION}:${AWS_ACCOUNT}:log-group:/aws/application-signals/data:*"
        ],
        "Condition": {
          "ArnLike": { "aws:SourceArn": "arn:aws:xray:${REGION}:${AWS_ACCOUNT}:*" },
          "StringEquals": { "aws:SourceAccount": "${AWS_ACCOUNT}" }
        }
      }
    ]
}
EOF
     )
     TXN_ERR=$(
          aws logs put-resource-policy --policy-name TransactionSearchXRayAccess \
               --policy-document "${TXN_POLICY}" --region "${REGION}" 2>&1 >/dev/null &&
               aws xray update-trace-segment-destination --destination CloudWatchLogs --region "${REGION}" 2>&1 >/dev/null
     ) || TXN_RC=1
     if [ -z "${TXN_RC:-}" ]; then
          log "Transaction Search enabled"
     else
          logwarn "Could not enable Transaction Search:"
          logwarn "${TXN_ERR}"
          logwarn "${TXN_SEARCH_DOC}"
     fi
}

# =============================================================================
# AWS EC2 trust
# =============================================================================

trust_aws_ec2() {
     if [ -z "${INSTANCE_ID}" ]; then usage; fi

     EC2_ROLE_TRUST='{
    "Effect": "Allow",
    "Principal": { "Service": "ec2.amazonaws.com" },
    "Action": "sts:AssumeRole"
  }'

     # An instance with a profile keeps it: swapping would revoke the role's
     # other permissions, which this script can't know about. So attach the
     # policy to the existing role instead. That modifies a pre-existing role
     # this script didn't create, so it requires explicit opt-in. The role
     # already trusts ec2.amazonaws.com, so no trust change is needed.
     CURRENT_PROFILE_ARN=$(aws ec2 describe-iam-instance-profile-associations \
          --filters "Name=instance-id,Values=${INSTANCE_ID}" "Name=state,Values=associated" \
          --query 'IamInstanceProfileAssociations[0].IamInstanceProfile.Arn' \
          --region "${REGION}" --output text 2>/dev/null || true)

     if [ -n "${CURRENT_PROFILE_ARN}" ] && [ "${CURRENT_PROFILE_ARN}" != "None" ]; then
          PROFILE_NAME="${CURRENT_PROFILE_ARN##*/}"
          EXISTING_ROLE=$(aws iam get-instance-profile \
               --instance-profile-name "${PROFILE_NAME}" \
               --query 'InstanceProfile.Roles[0].RoleName' --output text 2>/dev/null || true)

          if [ -z "${EXISTING_ROLE}" ] || [ "${EXISTING_ROLE}" = "None" ]; then
               die "Instance profile ${PROFILE_NAME} has no role attached"
          fi

          section "Using existing instance profile..."
          log "Instance profile ${PROFILE_NAME} attached to ${INSTANCE_ID}"
          log "Role: ${EXISTING_ROLE}"
          ROLE_NAME="${EXISTING_ROLE}"

          # Nothing to do if the policy is already there, so a re-run stays quiet
          # and needs no opt-in.
          if aws iam list-attached-role-policies --role-name "${ROLE_NAME}" \
               --query 'AttachedPolicies[?PolicyName==`CloudWatchAgentServerPolicy`] | [0].PolicyName' \
               --output text 2>/dev/null | grep -q CloudWatchAgentServerPolicy; then
               log "Managed policy CloudWatchAgentServerPolicy already attached"
               return
          fi

          if [ -t 0 ] && ! is_true "${EMIT_ENV}" && ! is_true "${UPDATE_INSTANCE_ROLE}"; then
               ask "Attach CloudWatchAgentServerPolicy to ${ROLE_NAME}? [y/N]"
               read -r answer || answer=""
               case "${answer}" in [yY]*) UPDATE_INSTANCE_ROLE="true" ;; esac
          fi
          if ! is_true "${UPDATE_INSTANCE_ROLE}"; then
               die "${ROLE_NAME} is missing CloudWatchAgentServerPolicy. Attach it manually, or set CWAGENT_AWS_UPDATE_INSTANCE_ROLE=true to have this script attach it"
          fi

          attach_permissions_policy
          return
     fi

     # No profile attached: create the role, its instance profile, and associate.
     PROFILE_NAME="${ROLE_NAME}"

     section "Configuring IAM role..."
     ensure_iam_role "${EC2_ROLE_TRUST}"
     attach_permissions_policy

     section "Configuring instance profile..."
     ensure_instance_profile "${PROFILE_NAME}"
     logaction "Associating instance profile with ${INSTANCE_ID}"
     aws ec2 associate-iam-instance-profile \
          --instance-id "${INSTANCE_ID}" \
          --iam-instance-profile Name="${PROFILE_NAME}" --region "${REGION}" >/dev/null
}

# Create and bind the instance profile if absent. The sleep covers IAM's
# eventual consistency before association.
ensure_instance_profile() {
     profile="$1"
     if aws iam get-instance-profile --instance-profile-name "${profile}" >/dev/null 2>&1; then
          log "Instance profile ${profile} exists"
          return
     fi
     logaction "Creating instance profile ${profile}"
     aws iam create-instance-profile --instance-profile-name "${profile}" >/dev/null
     aws iam add-role-to-instance-profile \
          --instance-profile-name "${profile}" --role-name "${ROLE_NAME}" >/dev/null
     logaction "Waiting for propagation..."
     sleep 10
}

# =============================================================================
# AWS ECS trust
# =============================================================================

trust_aws_ecs() {
     section "Configuring IAM task role..."

     ensure_iam_role '{
    "Effect": "Allow",
    "Principal": { "Service": "ecs-tasks.amazonaws.com" },
    "Action": "sts:AssumeRole"
  }'

     attach_permissions_policy
}

# =============================================================================
# AWS EKS trust
# =============================================================================

trust_aws_eks() {
     if [ -z "${CLUSTER_NAME}" ]; then usage; fi

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
          --namespace "${K8S_NAMESPACE}" \
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
          logaction "Creating association for ${K8S_NAMESPACE}/cloudwatch-agent"
          aws eks create-pod-identity-association \
               --cluster-name "${CLUSTER_NAME}" \
               --region "${REGION}" \
               --namespace "${K8S_NAMESPACE}" \
               --service-account cloudwatch-agent \
               --role-arn "${ROLE_ARN}" >/dev/null
     fi
}

# =============================================================================
# Azure VM trust
# =============================================================================

trust_azure_vm() {
     if [ -z "${TENANT_ID}" ]; then
          die "CWAGENT_AZURE_TENANT_ID is required for azure_vm (produced by azure/setup.sh)"
     fi

     OIDC_AUDIENCE="https://management.azure.com/"

     section "Configuring AWS trust..."

     ensure_oidc_provider "sts.windows.net/${TENANT_ID}/" \
          "https://sts.windows.net/${TENANT_ID}/" \
          "${OIDC_AUDIENCE}" \
          "626d44e704d1ceabe3bf0d53397464ac8080142c"

     TRUST_STATEMENT=$(
          cat <<EOF
{
    "Effect": "Allow",
    "Principal": {
      "Federated": "arn:aws:iam::${AWS_ACCOUNT}:oidc-provider/sts.windows.net/${TENANT_ID}/"
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
}

# =============================================================================
# Azure AKS trust
# =============================================================================

trust_azure_aks() {
     if [ -z "${OIDC_ISSUER}" ]; then
          die "CWAGENT_AZURE_OIDC_ISSUER is required for azure_aks (produced by azure/setup.sh)"
     fi

     OIDC_HOST="${OIDC_ISSUER#https://}"

     section "Configuring AWS trust..."

     ensure_oidc_provider "${OIDC_HOST}" "${OIDC_ISSUER}" sts.amazonaws.com

     TRUST_STATEMENT=$(
          cat <<EOF
{
    "Effect": "Allow",
    "Principal": {
      "Federated": "arn:aws:iam::${AWS_ACCOUNT}:oidc-provider/${OIDC_HOST}"
    },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": {
        "${OIDC_HOST}:sub": "system:serviceaccount:${K8S_NAMESPACE}:cloudwatch-agent",
        "${OIDC_HOST}:aud": "sts.amazonaws.com"
      }
    }
  }
EOF
     )

     ensure_iam_role "${TRUST_STATEMENT}"
     attach_permissions_policy
}

# =============================================================================
# GCP Compute Engine trust
# =============================================================================

trust_gcp_vm() {
     if [ -z "${SA_UNIQUE_ID}" ]; then
          die "CWAGENT_GCP_SA_UNIQUE_ID is required for gcp_vm (produced by gcp/setup.sh)"
     fi

     section "Configuring AWS trust..."

     # Google is a built-in web-identity provider (https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_principal.html#principal-federated-web-identity),
     # so unlike the Azure platforms no IAM OIDC provider resource is registered:
     # the principal is accounts.google.com itself. On Google identity tokens IAM
     # matches :sub against the service account's unique ID, :oaud against the
     # audience the token was requested with, and :aud against the authorized
     # party (azp), which on service-account tokens is the unique ID again.
     # Pinning all three follows the recommended trust policy for Google-issued
     # tokens: https://aws.amazon.com/blogs/security/access-aws-using-a-google-cloud-platform-native-workload-identity/.
     TRUST_STATEMENT=$(
          cat <<EOF
{
    "Effect": "Allow",
    "Principal": {
      "Federated": "accounts.google.com"
    },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": {
        "accounts.google.com:aud": "${SA_UNIQUE_ID}",
        "accounts.google.com:sub": "${SA_UNIQUE_ID}",
        "accounts.google.com:oaud": "sts.amazonaws.com"
      }
    }
  }
EOF
     )

     ensure_iam_role "${TRUST_STATEMENT}"
     attach_permissions_policy
}

# =============================================================================
# GCP GKE trust
# =============================================================================

trust_gcp_gke() {
     if [ -z "${GCP_OIDC_ISSUER}" ]; then
          die "CWAGENT_GCP_OIDC_ISSUER is required for gcp_gke (produced by gcp/setup.sh)"
     fi

     OIDC_HOST="${GCP_OIDC_ISSUER#https://}"

     section "Configuring AWS trust..."

     ensure_oidc_provider "${OIDC_HOST}" "${GCP_OIDC_ISSUER}" sts.amazonaws.com

     TRUST_STATEMENT=$(
          cat <<EOF
{
    "Effect": "Allow",
    "Principal": {
      "Federated": "arn:aws:iam::${AWS_ACCOUNT}:oidc-provider/${OIDC_HOST}"
    },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": {
        "${OIDC_HOST}:sub": "system:serviceaccount:${K8S_NAMESPACE}:cloudwatch-agent",
        "${OIDC_HOST}:aud": "sts.amazonaws.com"
      }
    }
  }
EOF
     )

     ensure_iam_role "${TRUST_STATEMENT}"
     attach_permissions_policy
}

# =============================================================================
# Install payload builders (EC2 path)
# =============================================================================

# Emit the one-line command that fetches install.sh and runs it on a Linux target.
# SSM RunCommand sets AWS_SHARED_CREDENTIALS_FILE to the DHMC role's credentials,
# which the SDK prefers over the instance profile we just attached; unset it so
# config-downloader uses the instance profile (which has CloudWatchAgentServerPolicy).
# No-op on non-DHMC instances, where the variable is not set.
linux_install_cmd() {
     printf 'unset AWS_SHARED_CREDENTIALS_FILE; curl -fsSL %s/install.sh | sh' "${SCRIPT_BASE_URL}"
}

# Windows counterpart. The fetch is wrapped into one -EncodedCommand base64
# payload (UTF-16LE) so the outer command needs no quoting through SSM.
# Remove-Item Env:AWS_SHARED_CREDENTIALS_FILE for the same reason as the Linux
# path: SSM sets it to the DHMC role's creds, which shadow the instance profile.
windows_install_cmd() {
     ps_script="Remove-Item Env:AWS_SHARED_CREDENTIALS_FILE -ErrorAction SilentlyContinue; Invoke-WebRequest -Uri ${SCRIPT_BASE_URL}/install.ps1 -OutFile \$env:TEMP\\cwagent-install.ps1; & \$env:TEMP\\cwagent-install.ps1"
     encoded=$(printf '%s' "${ps_script}" | iconv -f utf-8 -t utf-16le 2>/dev/null | base64 | tr -d '\n')
     [ -n "${encoded}" ] || return 1
     printf 'powershell -NoProfile -EncodedCommand %s' "${encoded}"
}

# Run a command on an EC2 instance via SSM. $1 = document name
# (AWS-RunShellScript | AWS-RunPowerShellScript), $2 = command.
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

     printf '%s\n' "${SSM_OUTPUT}" >&3
     if [ "${SSM_STATUS_DETAIL}" != "Success" ]; then
          aws ssm get-command-invocation \
               --command-id "${COMMAND_ID}" \
               --instance-id "${INSTANCE_ID}" \
               --region "${REGION}" --query 'StandardErrorContent' --output text >&2
          die "SSM command finished with status: ${SSM_STATUS_DETAIL}"
     fi
}

# =============================================================================
# AWS EC2 install
# =============================================================================

install_aws_ec2() {
     if [ -z "${INSTANCE_ID}" ]; then usage; fi

     INSTANCE_PLATFORM=$(aws ec2 describe-instances \
          --instance-ids "${INSTANCE_ID}" \
          --region "${REGION}" \
          --query 'Reservations[0].Instances[0].Platform' --output text 2>/dev/null || true)

     SSM_STATUS=$(aws ssm describe-instance-information \
          --filters "Key=InstanceIds,Values=${INSTANCE_ID}" \
          --query 'InstanceInformationList[0].PingStatus' \
          --region "${REGION}" --output text 2>/dev/null || true)

     if [ "${SSM_STATUS}" = "Online" ]; then
          section "Installing agent on ${INSTANCE_ID}..."
          if [ "${INSTANCE_PLATFORM}" = "windows" ]; then
               if INSTALL_CMD=$(windows_install_cmd); then
                    run_via_ssm "AWS-RunPowerShellScript" "${INSTALL_CMD}"
                    log "Agent installed on ${INSTANCE_ID}"
                    return
               fi
          else
               if INSTALL_CMD=$(linux_install_cmd); then
                    run_via_ssm "AWS-RunShellScript" "${INSTALL_CMD}"
                    log "Agent installed on ${INSTANCE_ID}"
                    return
               fi
          fi
          logwarn "could not build the install command (iconv is required for Windows targets)"
     else
          logwarn "SSM agent is not available on ${INSTANCE_ID}"
     fi

     printf '\n' >&3
     printf 'Done. Run the following on %s to install and start the agent:\n' "${INSTANCE_ID}" >&3
     printf '\n' >&3
     if [ "${INSTANCE_PLATFORM}" = "windows" ]; then
          printf '%s\n' "  # PowerShell, as Administrator:" >&3
          printf '%s\n' "  Invoke-WebRequest -Uri ${SCRIPT_BASE_URL}/install.ps1 -OutFile \$env:TEMP\\install.ps1; & \$env:TEMP\\install.ps1" >&3
     else
          printf '%s\n' "  curl -fsSL ${SCRIPT_BASE_URL}/install.sh | sudo sh" >&3
     fi
}

# =============================================================================
# AWS EKS install
# =============================================================================

install_aws_eks() {
     if [ -z "${CLUSTER_NAME}" ]; then usage; fi

     # The add-on injects k8sMode and clusterName, so configuration-values carries
     # only the pipeline config: OTel Container Insights + logs on, v1 CI/FluentBit
     # off, and the node + cluster-scraper agents.
     ADDON_CONFIG='{"containerInsights":{"enabled":false},"containerLogs":{"enabled":false},"otelContainerInsights":{"enabled":true,"logs":{"enabled":true}},"agents":[{"name":"cloudwatch-agent","config":"default:otel"},{"name":"cloudwatch-agent-cluster-scraper","mode":"deployment","config":"default"}]}'

     section "Installing the CloudWatch Observability add-on on ${CLUSTER_NAME}..."
     # create-addon for a new install, update-addon if it already exists.
     # --resolve-conflicts is left at its NONE default, so a re-run never
     # silently overwrites a value changed by hand on the add-on.
     if aws eks describe-addon --cluster-name "${CLUSTER_NAME}" --addon-name amazon-cloudwatch-observability --region "${REGION}" >/dev/null 2>&1; then
          logaction "Updating add-on configuration"
          aws eks update-addon \
               --cluster-name "${CLUSTER_NAME}" \
               --addon-name amazon-cloudwatch-observability \
               --configuration-values "${ADDON_CONFIG}" \
               --region "${REGION}" >/dev/null
     else
          logaction "Creating add-on"
          aws eks create-addon \
               --cluster-name "${CLUSTER_NAME}" \
               --addon-name amazon-cloudwatch-observability \
               --configuration-values "${ADDON_CONFIG}" \
               --region "${REGION}" >/dev/null
     fi
     # create/update is async. Wait for active, but a slow activation shouldn't
     # fail the run, so on timeout just point at the status command.
     if aws eks wait addon-active --cluster-name "${CLUSTER_NAME}" --addon-name amazon-cloudwatch-observability --region "${REGION}" 2>/dev/null; then
          log "Add-on active on ${CLUSTER_NAME}"
     else
          logwarn "Add-on submitted, still activating. Check: aws eks describe-addon --cluster-name ${CLUSTER_NAME} --addon-name amazon-cloudwatch-observability --region ${REGION}"
     fi
}

# =============================================================================
# AWS ECS install
#
# The ECS "install" is a container added to an existing task definition, so this
# prints that definition rather than deploying anything.
# =============================================================================

install_aws_ecs() {
     ECS_LAUNCH_TYPE="${ECS_LAUNCH_TYPE:-fargate}"

     section "Add this container to the task definition's containerDefinitions:"
     printf '\n' >&3
     cat >&3 <<EOF
    {
      "name": "cloudwatch-agent",
      "image": "public.ecr.aws/cloudwatch-agent/cloudwatch-agent:latest",
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

     printf '\n' >&3
     printf 'Also set on the task definition:\n' >&3
     if [ "${ECS_LAUNCH_TYPE}" = "ec2" ]; then
          printf '%s\n' "  \"taskRoleArn\": \"${ROLE_ARN}\"," >&3
          printf '%s\n' "  \"networkMode\": \"bridge\"" >&3
          printf '\n' >&3
          printf '%s\n' "Add to the application container:" >&3
          printf '%s\n' "  \"links\": [\"cloudwatch-agent\"]" >&3
          printf '%s\n' "  \"environment\": [{\"name\": \"OTEL_EXPORTER_OTLP_ENDPOINT\", \"value\": \"http://cloudwatch-agent:4317\"}]" >&3
     else
          printf '%s\n' "  \"taskRoleArn\": \"${ROLE_ARN}\"" >&3
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
     aws_ec2 | aws_ecs | aws_eks | azure_vm | azure_aks | gcp_vm | gcp_gke) ;;
     *) die "unsupported platform: ${PLATFORM:-<unset>} (valid: aws_ec2, aws_ecs, aws_eks, azure_vm, azure_aks, gcp_vm, gcp_gke)" ;;
     esac

     check_prerequisites

     # Region is baked into the role/endpoint and is where telemetry lands, so
     # never guess it: fail rather than default silently.
     if [ -z "${REGION}" ]; then
          REGION=$(aws configure get region 2>/dev/null || true)
     fi
     if [ -z "${REGION}" ] && [ -t 0 ] && ! is_true "${EMIT_ENV}"; then
          prompt REGION "AWS region telemetry is sent to"
     fi
     [ -n "${REGION}" ] || die "CWAGENT_AWS_REGION is required (set it or run 'aws configure set region <region>')"

     # Trust always runs. The Azure and GCP platforms stop after emitting the
     # ARN (install happens on their own cloud side).
     case "${PLATFORM}" in
     aws_ec2) trust_aws_ec2 ;;
     aws_ecs) trust_aws_ecs ;;
     aws_eks) trust_aws_eks ;;
     azure_vm) trust_azure_vm ;;
     azure_aks) trust_azure_aks ;;
     gcp_vm) trust_gcp_vm ;;
     gcp_gke) trust_gcp_gke ;;
     esac

     ROLE_ARN=$(aws iam get-role \
          --role-name "${ROLE_NAME}" \
          --query Role.Arn --output text)
     log "Role ARN: ${ROLE_ARN}"

     ensure_transaction_search

     case "${PLATFORM}" in
     aws_ec2) install_aws_ec2 ;;
     aws_eks) install_aws_eks ;;
     aws_ecs) install_aws_ecs ;;
     azure_vm | azure_aks)
          log "Trust configured. Install runs on the Azure side (azure/setup.sh) with the role ARN above."
          ;;
     gcp_vm | gcp_gke)
          log "Trust configured. Install runs on the GCP side (gcp/setup.sh) with the role ARN above."
          ;;
     esac

     add_env CWAGENT_PLATFORM "${PLATFORM}"
     add_env CWAGENT_AWS_ROLE_ARN "${ROLE_ARN}"
     add_env CWAGENT_AWS_REGION "${REGION}"
     # Carried through so a later step (or a paste command) is self-contained,
     # with no values to re-enter by hand.
     add_env CWAGENT_AWS_INSTANCE_ID "${INSTANCE_ID}"
     add_env CWAGENT_K8S_CLUSTER_NAME "${CLUSTER_NAME}"

     emit_env
}

main "$@"
