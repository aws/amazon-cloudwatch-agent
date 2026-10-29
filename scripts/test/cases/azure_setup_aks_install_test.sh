#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# azure/setup.sh install mode (CWAGENT_AWS_ROLE_ARN set) on an AKS cluster: the
# chart is installed only when helm and kubectl are both on PATH. A missing tool
# prints the manual commands, names what is missing, and exits non-zero. A
# failing kubeconfig or helm step dies with a message naming the step. The tool
# check runs before any cloud call, in particular before the slow "az aks
# update". Every case hides the host's own helm and kubectl (hide_tools), so
# only the fakes a case makes exist.

set -u
TEST_DIR="${TEST_DIR:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}"
. "${TEST_DIR}/helpers.sh"

ROLE_ARN="arn:aws:iam::123456789012:role/CloudWatchAgentServerRole-test"
ISSUER="https://eastus.oic.prod-aks.azure.com/tenant-id/cluster-id/"

# Fake az: OIDC issuer and workload identity already enabled, so no update,
# unless the sandbox flag oidc_off is set. credentials_fail makes
# get-credentials fail.
az_fake() {
     make_fake az <<'EOF'
case "$*" in
"version"*) printf '2.60.0\n' ;;
"account show"*) printf 'sub-id\ttest-sub\ttenant-id\n' ;;
"aks show"*)
     if [ -f "${SANDBOX}/oidc_off" ]; then
          printf 'False\tFalse\t\n'
     else
          printf 'True\tTrue\thttps://eastus.oic.prod-aks.azure.com/tenant-id/cluster-id/\n'
     fi
     ;;
"aks get-credentials"*)
     if [ -f "${SANDBOX}/credentials_fail" ]; then
          printf 'ERROR: (AuthorizationFailed) not authorized\n' >&2
          exit 1
     fi
     printf 'Merged "test-cluster" as current context\n'
     ;;
*) : ;;
esac
EOF
}

helm_fake() {
     make_fake helm <<'EOF'
case "$*" in
"upgrade"*)
     [ -f "${SANDBOX}/upgrade_fail" ] && { printf 'Error: INSTALLATION FAILED: context deadline exceeded\n' >&2; exit 1; }
     printf 'STATUS: deployed\n'
     ;;
*) : ;;
esac
EOF
}

noop_fake() {
     make_fake "$1" <<'EOF'
:
EOF
}

HIDDEN="helm kubectl"

run_install() {
     hide_tools ${HIDDEN}
     run_setup azure/setup.sh \
          CWAGENT_PLATFORM=azure_aks \
          CWAGENT_AZURE_RESOURCE_GROUP=test-rg \
          CWAGENT_K8S_CLUSTER_NAME=test-cluster \
          CWAGENT_AWS_ROLE_ARN="${ROLE_ARN}" \
          CWAGENT_AWS_REGION=us-east-1
}

t_case "helm and kubectl present: chart installed, issuer reported"
az_fake
helm_fake
noop_fake kubectl
run_install
assert_status 0
assert_called "az aks get-credentials --resource-group test-rg --name test-cluster --overwrite-existing"
assert_called "helm upgrade --install amazon-cloudwatch-observability .*--set k8sMode=AKS --set roleArn=${ROLE_ARN}"
assert_output_contains stdout "Chart installed on 'test-cluster'"
assert_output_contains stdout "OIDC issuer (for the AWS setup): ${ISSUER}"
assert_output_lacks stdout "manually"

t_case "kubectl missing: manual commands printed, kubectl named, exit 1"
az_fake
helm_fake
run_install
assert_status 1
assert_output_contains stdout "Install the Amazon CloudWatch Observability Helm chart manually"
assert_output_contains stdout "--set k8sMode=AKS"
assert_output_contains stdout "--set-string 'agents[1].name=cloudwatch-agent-cluster-scraper'"
assert_output_contains stderr "Helm chart not installed on 'test-cluster': missing kubectl."
assert_output_lacks stdout "Chart installed"
assert_output_lacks stdout "OIDC issuer (for the AWS setup)"
# The check runs before any cluster call: nothing is read, changed, or configured.
assert_not_called "aks show"
assert_not_called "aks get-credentials"
assert_not_called "helm"

t_case "kubectl missing on a cluster without OIDC: fails before the slow aks update"
az_fake
helm_fake
: >"${SANDBOX}/oidc_off"
run_install
assert_status 1
assert_output_contains stderr "missing kubectl."
assert_not_called "aks update"
assert_output_lacks stdout "Enabling OIDC issuer"

t_case "helm and kubectl missing: both named"
az_fake
run_install
assert_status 1
assert_output_contains stderr "missing helm, kubectl."

t_case "get-credentials fails: dies naming the kubeconfig step"
az_fake
helm_fake
noop_fake kubectl
: >"${SANDBOX}/credentials_fail"
run_install
assert_status 1
assert_output_contains stderr "could not configure kubeconfig for 'test-cluster'"
assert_not_called "helm upgrade"

t_case "helm upgrade fails: dies naming the upgrade step"
az_fake
helm_fake
noop_fake kubectl
: >"${SANDBOX}/upgrade_fail"
run_install
assert_status 1
assert_output_contains stderr "helm upgrade failed on 'test-cluster'"
assert_output_lacks stdout "Chart installed"

t_case "identity mode (no ARN) without helm/kubectl: still succeeds"
az_fake
hide_tools ${HIDDEN}
run_setup azure/setup.sh \
     CWAGENT_EMIT_ENV=1 \
     CWAGENT_PLATFORM=azure_aks \
     CWAGENT_AZURE_RESOURCE_GROUP=test-rg \
     CWAGENT_K8S_CLUSTER_NAME=test-cluster
assert_status 0
assert_output_contains stdout "CWAGENT_AZURE_OIDC_ISSUER='${ISSUER}'"
assert_output_lacks stderr "missing"

t_end
