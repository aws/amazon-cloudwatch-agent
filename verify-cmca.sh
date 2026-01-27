#!/bin/bash
# CMCA Verification Script
# Builds and runs the cmca-verify tool to validate provider implementations

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== CMCA Provider Verification ==="
echo ""

# Build the verification tool
echo "Building cmca-verify tool..."
go build -o build/bin/cmca-verify ./cmd/cmca-verify

if [ ! -f "build/bin/cmca-verify" ]; then
    echo "❌ Failed to build cmca-verify"
    exit 1
fi

echo "✅ Build successful"
echo ""

# Run verification
echo "Running verification..."
echo ""

./build/bin/cmca-verify "$@"

exit_code=$?

if [ $exit_code -eq 0 ]; then
    echo "✅ All verifications passed!"
else
    echo "❌ Some verifications failed"
fi

exit $exit_code
