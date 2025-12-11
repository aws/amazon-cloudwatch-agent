#!/bin/bash

# Get latest commit SHA from aws-cwa-dev branch
LATEST_SHA=$(git ls-remote https://github.com/amazon-contributing/opentelemetry-collector-contrib.git aws-cwa-dev | awk '{print $1}')

echo "Updating to commit: $LATEST_SHA"

# Update only amazon-contributing references in replace directives
sed -i '' "s|=> github\.com/amazon-contributing/opentelemetry-collector-contrib/\([^@]*\) v0\.0\.0-[0-9]*-[a-f0-9]*|=> github.com/amazon-contributing/opentelemetry-collector-contrib/\1 $LATEST_SHA|g" go.mod

go mod tidy

echo "Updated all OTel dependencies to $LATEST_SHA"
