#!/bin/sh

# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: MIT

# gcp/setup.sh identity mode (no CWAGENT_AWS_ROLE_ARN): under CWAGENT_EMIT_ENV
# stdout must be exactly the eval-able KEY='value' lines, the GKE issuer URL is
# constructed from project/location/cluster, and identity failures die cleanly.

set -u
TEST_DIR="${TEST_DIR:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}"
. "${TEST_DIR}/helpers.sh"

SA_ID="107826487864658064577"

gcloud_fake() {
     make_fake gcloud <<'EOF'
case "$*" in
*"auth list"*) printf 'tester@example.com\n' ;;
*"projects describe"*) printf 'test-project\n' ;;
*"compute instances describe"*)
     if [ -f "${SANDBOX}/instance_has_no_sa" ]; then
          printf '\n'
     else
          printf 'test-sa@test-project.iam.gserviceaccount.com\n'
     fi
     ;;
*"iam service-accounts describe"*) printf '107826487864658064577\n' ;;
*"container clusters describe"*) printf 'test-cluster\n' ;;
*) : ;;
esac
EOF
}

t_case "gcp_gce emit: stdout is exactly the five KEY='value' lines"
gcloud_fake
run_setup gcp/setup.sh \
     CWAGENT_EMIT_ENV=1 \
     CWAGENT_PLATFORM=gcp_gce \
     CWAGENT_GCP_PROJECT=test-project \
     CWAGENT_GCP_LOCATION=us-east1-b \
     CWAGENT_GCP_INSTANCE_NAME=test-vm
assert_status 0
assert_stdout_exact <<EOF
CWAGENT_PLATFORM='gcp_gce'
CWAGENT_GCP_PROJECT='test-project'
CWAGENT_GCP_LOCATION='us-east1-b'
CWAGENT_GCP_INSTANCE_NAME='test-vm'
CWAGENT_GCP_SA_UNIQUE_ID='${SA_ID}'
EOF
assert_output_contains stderr "install pending the IAM role ARN"
assert_output_contains stderr "Service account unique ID (for the AWS setup): ${SA_ID}"

t_case "gcp_gce emit lines are eval-able"
gcloud_fake
run_setup gcp/setup.sh \
     CWAGENT_EMIT_ENV=1 \
     CWAGENT_PLATFORM=gcp_gce \
     CWAGENT_GCP_PROJECT=test-project \
     CWAGENT_GCP_LOCATION=us-east1-b \
     CWAGENT_GCP_INSTANCE_NAME=test-vm
assert_status 0
CWAGENT_GCP_SA_UNIQUE_ID=""
eval "$(cat "${SANDBOX}/stdout")"
if [ "${CWAGENT_GCP_SA_UNIQUE_ID}" != "${SA_ID}" ]; then
     fail "eval of emitted lines did not set CWAGENT_GCP_SA_UNIQUE_ID"
fi

t_case "gcp_gke emit: issuer URL constructed from project, location, and cluster"
gcloud_fake
run_setup gcp/setup.sh \
     CWAGENT_EMIT_ENV=1 \
     CWAGENT_PLATFORM=gcp_gke \
     CWAGENT_GCP_PROJECT=test-project \
     CWAGENT_GCP_LOCATION=us-east1 \
     CWAGENT_K8S_CLUSTER_NAME=test-cluster
assert_status 0
assert_stdout_exact <<EOF
CWAGENT_PLATFORM='gcp_gke'
CWAGENT_GCP_PROJECT='test-project'
CWAGENT_GCP_LOCATION='us-east1'
CWAGENT_K8S_CLUSTER_NAME='test-cluster'
CWAGENT_GCP_OIDC_ISSUER='https://container.googleapis.com/v1/projects/test-project/locations/us-east1/clusters/test-cluster'
EOF
assert_output_contains stderr "OIDC issuer (for the AWS setup)"

t_case "instance without a service account: dies with the attach remedy"
gcloud_fake
: >"${SANDBOX}/instance_has_no_sa"
run_setup gcp/setup.sh \
     CWAGENT_EMIT_ENV=1 \
     CWAGENT_PLATFORM=gcp_gce \
     CWAGENT_GCP_PROJECT=test-project \
     CWAGENT_GCP_LOCATION=us-east1-b \
     CWAGENT_GCP_INSTANCE_NAME=test-vm
assert_status 1
assert_output_contains stderr "no service account attached to 'test-vm'"
assert_output_contains stderr "set-service-account"

t_case "unsupported platform: dies listing the valid values"
gcloud_fake
run_setup gcp/setup.sh CWAGENT_EMIT_ENV=1 CWAGENT_PLATFORM=gcp_bogus
assert_status 1
assert_output_contains stderr "unsupported platform: gcp_bogus (valid: gcp_gce, gcp_gke)"

t_end
