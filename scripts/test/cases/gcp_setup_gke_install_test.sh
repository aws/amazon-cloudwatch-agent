#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# gcp/setup.sh install mode (CWAGENT_AWS_ROLE_ARN set) on a GKE cluster: the
# chart is installed only when helm, kubectl, and gke-gcloud-auth-plugin are all
# on PATH. A missing tool prints the manual commands, names what is missing, and
# exits non-zero, so the run never reads as a successful install. A failing
# kubeconfig or helm step dies with a message naming the step. The tool check
# runs before any cloud call. Every case hides the host's own helm, kubectl, and
# gke-gcloud-auth-plugin (hide_tools), so only the fakes a case makes exist.

set -u
TEST_DIR="${TEST_DIR:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}"
. "${TEST_DIR}/helpers.sh"

ROLE_ARN="arn:aws:iam::123456789012:role/CloudWatchAgentServerRole-test"
ISSUER="https://container.googleapis.com/v1/projects/test-project/locations/us-east1/clusters/test-cluster"

# Fake gcloud. Sandbox flag credentials_fail makes get-credentials fail.
gcloud_fake() {
     make_fake gcloud <<'EOF'
case "$*" in
*"auth list"*) printf 'tester@example.com\n' ;;
*"projects describe"*) printf 'test-project\n' ;;
*"container clusters describe"*) printf 'test-cluster\n' ;;
*"container clusters get-credentials"*)
     if [ -f "${SANDBOX}/credentials_fail" ]; then
          printf 'ERROR: (gcloud.container.clusters.get-credentials) permission denied\n' >&2
          exit 1
     fi
     printf 'kubeconfig entry generated for test-cluster.\n' >&2
     ;;
*) : ;;
esac
EOF
}

# Fake helm. Sandbox flags repo_add_fail and upgrade_fail fail those steps.
helm_fake() {
     make_fake helm <<'EOF'
case "$*" in
"repo add"*)
     [ -f "${SANDBOX}/repo_add_fail" ] && { printf 'Error: looks like the URL is not a valid chart repository\n' >&2; exit 1; }
     printf '"aws-observability" has been added to your repositories\n'
     ;;
"repo update"*) printf 'Update Complete.\n' ;;
"upgrade"*)
     [ -f "${SANDBOX}/upgrade_fail" ] && { printf 'Error: INSTALLATION FAILED: context deadline exceeded\n' >&2; exit 1; }
     printf 'STATUS: deployed\n'
     ;;
*) : ;;
esac
EOF
}

# A present-but-inert tool: only its presence on PATH matters.
noop_fake() {
     make_fake "$1" <<'EOF'
:
EOF
}

all_tools() {
     gcloud_fake
     helm_fake
     noop_fake kubectl
     noop_fake gke-gcloud-auth-plugin
}

HIDDEN="helm kubectl gke-gcloud-auth-plugin"

run_install() {
     hide_tools ${HIDDEN}
     run_setup gcp/setup.sh \
          CWAGENT_PLATFORM=gcp_gke \
          CWAGENT_GCP_PROJECT=test-project \
          CWAGENT_GCP_LOCATION=us-east1 \
          CWAGENT_K8S_CLUSTER_NAME=test-cluster \
          CWAGENT_AWS_ROLE_ARN="${ROLE_ARN}" \
          CWAGENT_AWS_REGION=us-east-1
}

t_case "all tools present: chart installed, issuer reported"
all_tools
run_install
assert_status 0
assert_called "gcloud container clusters get-credentials test-cluster --location us-east1 --project test-project"
assert_called "helm repo add --force-update aws-observability https://aws-observability.github.io/helm-charts"
assert_called "helm upgrade --install amazon-cloudwatch-observability .*--set k8sMode=GKE --set roleArn=${ROLE_ARN} --set region=us-east-1 --set clusterName=test-cluster"
assert_output_contains stdout "Chart installed on 'test-cluster'"
assert_output_contains stdout "OIDC issuer (for the AWS setup): ${ISSUER}"
assert_output_lacks stdout "manually"

t_case "kubectl missing: manual commands printed, kubectl named, exit 1"
gcloud_fake
helm_fake
noop_fake gke-gcloud-auth-plugin
run_install
assert_status 1
assert_output_contains stdout "Install the Amazon CloudWatch Observability Helm chart manually"
assert_output_contains stdout "--set roleArn=${ROLE_ARN}"
assert_output_contains stdout "--set-string 'agents[1].name=cloudwatch-agent-cluster-scraper'"
assert_output_contains stderr "Helm chart not installed on 'test-cluster': missing kubectl."
assert_output_lacks stdout "Chart installed"
assert_output_lacks stdout "OIDC issuer (for the AWS setup)"
# The check runs before any cloud call: nothing is read or configured.
assert_not_called "container clusters describe"
assert_not_called "get-credentials"
assert_not_called "helm"

t_case "every tool missing: all three named in order"
gcloud_fake
run_install
assert_status 1
assert_output_contains stderr "missing helm, kubectl, gke-gcloud-auth-plugin."

t_case "only gke-gcloud-auth-plugin missing: it is named alone"
gcloud_fake
helm_fake
noop_fake kubectl
run_install
assert_status 1
assert_output_contains stderr "missing gke-gcloud-auth-plugin."
assert_output_lacks stderr "missing helm"

t_case "get-credentials fails: dies naming the kubeconfig step"
all_tools
: >"${SANDBOX}/credentials_fail"
run_install
assert_status 1
assert_output_contains stdout "permission denied"
assert_output_contains stderr "could not configure kubeconfig for 'test-cluster'"
assert_not_called "helm upgrade"
assert_output_lacks stdout "Chart installed"

t_case "helm repo add fails: dies naming the repo step"
all_tools
: >"${SANDBOX}/repo_add_fail"
run_install
assert_status 1
assert_output_contains stderr "helm repo add failed for https://aws-observability.github.io/helm-charts"
assert_not_called "helm upgrade"

t_case "helm upgrade fails: dies naming the upgrade step"
all_tools
: >"${SANDBOX}/upgrade_fail"
run_install
assert_status 1
assert_output_contains stderr "INSTALLATION FAILED"
assert_output_contains stderr "helm upgrade failed on 'test-cluster'"
assert_output_lacks stdout "Chart installed"

t_case "identity mode (no ARN) without helm/kubectl: still succeeds"
gcloud_fake
hide_tools ${HIDDEN}
run_setup gcp/setup.sh \
     CWAGENT_EMIT_ENV=1 \
     CWAGENT_PLATFORM=gcp_gke \
     CWAGENT_GCP_PROJECT=test-project \
     CWAGENT_GCP_LOCATION=us-east1 \
     CWAGENT_K8S_CLUSTER_NAME=test-cluster
assert_status 0
assert_output_contains stdout "CWAGENT_GCP_OIDC_ISSUER='${ISSUER}'"
assert_output_lacks stderr "missing"

t_end
