#!/bin/bash

# Change to script directory
cd "$(dirname "$0")" || exit

# Get latest release tag from GitHub API
LATEST_VERSION=$(curl -s "https://api.github.com/repos/open-telemetry/opentelemetry-java-contrib/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

# Download the JAR
curl -L -o "opentelemetry-jmx-metrics.jar" "https://github.com/open-telemetry/opentelemetry-java-contrib/releases/download/${LATEST_VERSION}/opentelemetry-jmx-metrics.jar"

echo "Downloaded: [Version: ${LATEST_VERSION}] opentelemetry-jmx-metrics.jar"
