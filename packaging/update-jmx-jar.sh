#!/bin/bash
set -euo pipefail

# Change to script directory
cd "$(dirname "$0")" || exit

# Get latest release tag from GitHub API
LATEST_VERSION=$(curl -s "https://api.github.com/repos/open-telemetry/opentelemetry-java-contrib/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

# Download to a temp file first; only install after the allow-list check passes.
TMP_JAR=$(mktemp /tmp/opentelemetry-jmx-metrics.XXXXXX.jar)
trap 'rm -f "${TMP_JAR}"' EXIT
curl -L -o "${TMP_JAR}" "https://github.com/open-telemetry/opentelemetry-java-contrib/releases/download/${LATEST_VERSION}/opentelemetry-jmx-metrics.jar"

JAR_SHA=$(sha256sum "${TMP_JAR}" | cut -d' ' -f1)

# The jmxreceiver hash-gates the JAR at runtime against its supported_jars.go
# allow-list: a JAR whose sha256 is not listed makes the receiver launch a broken
# java invocation that fails and restarts every ~5s (silent JMX metric loss). Only
# accept a JAR whose hash the pinned receiver version supports.
JMX_RECEIVER_DIR=$( (
     cd .. && go mod download github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver >/dev/null 2>&1
     go list -m -f '{{.Dir}}' github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver
))
SUPPORTED_JARS="${JMX_RECEIVER_DIR}/supported_jars.go"
if [ ! -f "${SUPPORTED_JARS}" ]; then
     echo "ERROR: cannot locate supported_jars.go for the pinned jmxreceiver (looked in ${JMX_RECEIVER_DIR})" >&2
     exit 1
fi
if ! grep -q "${JAR_SHA}" "${SUPPORTED_JARS}"; then
     echo "ERROR: ${LATEST_VERSION} jar (sha256 ${JAR_SHA}) is NOT in the pinned jmxreceiver's allow-list:" >&2
     echo "  ${SUPPORTED_JARS}" >&2
     echo "Shipping it would break JMX at runtime (unsupported-jar restart loop)." >&2
     echo "Bump the fork pin to a version that supports this jar first, or pick a supported jar version." >&2
     exit 1
fi

mv "${TMP_JAR}" "opentelemetry-jmx-metrics.jar"
trap - EXIT

echo "Downloaded: [Version: ${LATEST_VERSION}] opentelemetry-jmx-metrics.jar (sha256 ${JAR_SHA}, allow-listed)"
echo "Remember to update the pinned hash in packaging/opentelemetry_jmx_jar_test.go"
