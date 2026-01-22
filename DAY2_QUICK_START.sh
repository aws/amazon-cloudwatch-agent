#!/bin/bash
# Day 2 Runtime Testing - Quick Start
# Branch: paramadon/multi-cloud-instanceId

set -e

echo "=== Day 2: Runtime Testing ==="
echo ""

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo "Error: Must run from amazon-cloudwatch-agent directory"
    exit 1
fi

# Build agent
echo "1. Building agent..."
make build
echo "✓ Build complete"
echo ""

# Test 1: Cloud-agnostic on current platform
echo "2. Testing cloud-agnostic placeholders..."
./build/bin/linux_amd64/config-translator \
  -input test-configs/placeholder-test.json \
  -output /tmp/placeholder-test.toml \
  -mode ec2 \
  -os linux

echo "✓ Config translated"
echo ""

echo "3. Checking translated config..."
echo "Namespace:"
grep "namespace" /tmp/placeholder-test.toml | head -1

echo ""
echo "Dimensions:"
grep -A 10 "append_dimensions" /tmp/placeholder-test.toml | grep -E "InstanceId|Region|AccountId|Hostname|PrivateIP"

echo ""
echo "=== Ready to run agent ==="
echo ""
echo "Run agent with:"
echo "  sudo ./build/bin/linux_amd64/amazon-cloudwatch-agent \\"
echo "    -config /tmp/placeholder-test.toml \\"
echo "    -envconfig <(echo '{}') \\"
echo "    2>&1 | tee /tmp/agent-output.log"
echo ""
echo "Then verify:"
echo "  grep 'Cloud metadata provider ready' /tmp/agent-output.log"
echo "  grep 'Resolved cloud-agnostic placeholders' /tmp/agent-output.log"
echo ""
echo "Compare with IMDS:"
echo "  # AWS:"
echo "  curl -s http://169.254.169.254/latest/meta-data/instance-id"
echo "  # Azure:"
echo "  curl -s -H 'Metadata: true' 'http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01&format=json' | jq .vmId"
