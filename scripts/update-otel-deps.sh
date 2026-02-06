#!/bin/bash

# Accept commit SHA as parameter, or get latest if not provided
if [ -n "$1" ]; then
     LATEST_SHA="$1"
     echo "Using provided commit SHA: $LATEST_SHA"
else
     # Get latest commit SHA from aws-cwa-dev branch
     LATEST_SHA=$(git ls-remote https://github.com/amazon-contributing/opentelemetry-collector-contrib.git aws-cwa-dev | awk '{print $1}')
     echo "Fetched latest commit SHA: $LATEST_SHA"
fi

# Check if go.mod exists before proceeding
if [ ! -f "go.mod" ]; then
     echo "Error: go.mod not found in current directory"
     exit 1
fi

echo "Updating go.mod with SHA: $LATEST_SHA"

# Update only amazon-contributing references in replace directives
sed -i '' "s|=> github\.com/amazon-contributing/opentelemetry-collector-contrib/\([^@]*\) v0\.0\.0-[0-9]*-[a-f0-9]*|=> github.com/amazon-contributing/opentelemetry-collector-contrib/\1 $LATEST_SHA|g" go.mod

echo "Running go mod tidy..."
GOPROXY=direct go mod tidy

echo "Updated all OTel dependencies to $LATEST_SHA"
