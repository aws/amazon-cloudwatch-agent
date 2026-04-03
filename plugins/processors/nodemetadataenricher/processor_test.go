// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadataenricher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/nodemetadatacache"
)

// setupCacheWithTestData creates a NodeMetadataCache, sets it as the singleton,
// and populates it with the provided node→metadata entries.
func setupCacheWithTestData(t *testing.T, entries map[string]*nodemetadatacache.NodeMetadata) {
	t.Helper()

	logger, _ := zap.NewDevelopment()
	cache := nodemetadatacache.NewForTest(logger)
	for nodeName, md := range entries {
		cache.SetForTest(nodeName, md)
	}
	nodemetadatacache.SetNodeMetadataCacheForTest(cache)

	t.Cleanup(func() {
		nodemetadatacache.SetNodeMetadataCacheForTest(nil)
	})
}

func newTestProcessor() *nodeMetadataEnricherProcessor {
	logger, _ := zap.NewDevelopment()
	return newNodeMetadataEnricherProcessor(logger)
}

func createTestMetrics(nodeName string, existingAttrs map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	if nodeName != "" {
		rm.Resource().Attributes().PutStr("k8s.node.name", nodeName)
	}
	for k, v := range existingAttrs {
		rm.Resource().Attributes().PutStr(k, v)
	}
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("kube_pod_info")
	return md
}

func freshMetadata() *nodemetadatacache.NodeMetadata {
	return &nodemetadatacache.NodeMetadata{
		HostID:           "i-0abc111def222",
		HostName:         "ip-10-0-1-42.ec2.internal",
		HostType:         "m5.xlarge",
		HostImageID:      "ami-0123456789abcdef0",
		AvailabilityZone: "us-east-1a",
		RenewTime:        time.Now(),
		LeaseDuration:    300,
	}
}

func TestEnrichmentWithCacheHit(t *testing.T) {
	md := freshMetadata()
	setupCacheWithTestData(t, map[string]*nodemetadatacache.NodeMetadata{
		"node-1": md,
	})

	p := newTestProcessor()
	input := createTestMetrics("node-1", nil)

	output, err := p.processMetrics(context.Background(), input)
	require.NoError(t, err)

	attrs := output.ResourceMetrics().At(0).Resource().Attributes()
	hostID, ok := attrs.Get(attrHostID)
	require.True(t, ok, "host.id should be set")
	assert.Equal(t, "i-0abc111def222", hostID.Str())

	hostName, ok := attrs.Get(attrHostName)
	require.True(t, ok, "host.name should be set")
	assert.Equal(t, "ip-10-0-1-42.ec2.internal", hostName.Str())

	hostType, ok := attrs.Get(attrHostType)
	require.True(t, ok, "host.type should be set")
	assert.Equal(t, "m5.xlarge", hostType.Str())

	imageID, ok := attrs.Get(attrHostImageID)
	require.True(t, ok, "host.image.id should be set")
	assert.Equal(t, "ami-0123456789abcdef0", imageID.Str())

	az, ok := attrs.Get(attrAvailabilityZone)
	require.True(t, ok, "cloud.availability_zone should be set")
	assert.Equal(t, "us-east-1a", az.Str())
}

func TestPassThroughWithCacheMiss(t *testing.T) {
	// Cache has data for node-1 but metric references node-2
	setupCacheWithTestData(t, map[string]*nodemetadatacache.NodeMetadata{
		"node-1": freshMetadata(),
	})

	p := newTestProcessor()
	input := createTestMetrics("node-2", nil)

	output, err := p.processMetrics(context.Background(), input)
	require.NoError(t, err)

	attrs := output.ResourceMetrics().At(0).Resource().Attributes()
	_, ok := attrs.Get(attrHostID)
	assert.False(t, ok, "host.id should NOT be set on cache miss")
	_, ok = attrs.Get(attrHostName)
	assert.False(t, ok, "host.name should NOT be set on cache miss")
	_, ok = attrs.Get(attrHostType)
	assert.False(t, ok, "host.type should NOT be set on cache miss")
	_, ok = attrs.Get(attrHostImageID)
	assert.False(t, ok, "host.image.id should NOT be set on cache miss")
	_, ok = attrs.Get(attrAvailabilityZone)
	assert.False(t, ok, "cloud.availability_zone should NOT be set on cache miss")
}

func TestPassThroughWithoutNodeName(t *testing.T) {
	setupCacheWithTestData(t, map[string]*nodemetadatacache.NodeMetadata{
		"node-1": freshMetadata(),
	})

	p := newTestProcessor()
	// No node name — simulates namespace/deployment/cluster-level KSM metrics
	input := createTestMetrics("", nil)

	output, err := p.processMetrics(context.Background(), input)
	require.NoError(t, err)

	attrs := output.ResourceMetrics().At(0).Resource().Attributes()
	_, ok := attrs.Get(attrHostID)
	assert.False(t, ok, "host.id should NOT be set when k8s.node.name is absent")
}

func TestPassThroughWithEmptyNodeName(t *testing.T) {
	setupCacheWithTestData(t, map[string]*nodemetadatacache.NodeMetadata{
		"": freshMetadata(), // even if cache has an entry for empty string
	})

	p := newTestProcessor()
	// Explicitly set k8s.node.name to empty string
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k8s.node.name", "")
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("kube_pod_info")

	output, err := p.processMetrics(context.Background(), md)
	require.NoError(t, err)

	attrs := output.ResourceMetrics().At(0).Resource().Attributes()
	_, ok := attrs.Get(attrHostID)
	assert.False(t, ok, "host.id should NOT be set when k8s.node.name is empty")
}

func TestMetricCountPreservation(t *testing.T) {
	setupCacheWithTestData(t, map[string]*nodemetadatacache.NodeMetadata{
		"node-1": freshMetadata(),
	})

	p := newTestProcessor()

	// Create metrics with 3 ResourceMetrics, each with 2 ScopeMetrics, each with 2 Metrics
	input := pmetric.NewMetrics()
	for i := 0; i < 3; i++ {
		rm := input.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("k8s.node.name", "node-1")
		for j := 0; j < 2; j++ {
			sm := rm.ScopeMetrics().AppendEmpty()
			for k := 0; k < 2; k++ {
				m := sm.Metrics().AppendEmpty()
				m.SetName("kube_pod_info")
			}
		}
	}

	output, err := p.processMetrics(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, 3, output.ResourceMetrics().Len(), "ResourceMetrics count should be preserved")
	for i := 0; i < output.ResourceMetrics().Len(); i++ {
		rm := output.ResourceMetrics().At(i)
		assert.Equal(t, 2, rm.ScopeMetrics().Len(), "ScopeMetrics count should be preserved")
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			assert.Equal(t, 2, rm.ScopeMetrics().At(j).Metrics().Len(), "Metrics count should be preserved")
		}
	}
}

func TestCloudAZOverwrite(t *testing.T) {
	setupCacheWithTestData(t, map[string]*nodemetadatacache.NodeMetadata{
		"node-1": freshMetadata(),
	})

	p := newTestProcessor()
	// Pre-set cloud.availability_zone to a wrong value (simulating resourcedetection)
	input := createTestMetrics("node-1", map[string]string{
		"cloud.availability_zone": "us-west-2b",
	})

	output, err := p.processMetrics(context.Background(), input)
	require.NoError(t, err)

	attrs := output.ResourceMetrics().At(0).Resource().Attributes()
	az, ok := attrs.Get(attrAvailabilityZone)
	require.True(t, ok)
	assert.Equal(t, "us-east-1a", az.Str(), "cloud.availability_zone should be overwritten with correct per-node value")
}

func TestMixedMetrics(t *testing.T) {
	setupCacheWithTestData(t, map[string]*nodemetadatacache.NodeMetadata{
		"node-1": freshMetadata(),
	})

	p := newTestProcessor()

	input := pmetric.NewMetrics()

	// RM 0: has node name with cache hit → should be enriched
	rm0 := input.ResourceMetrics().AppendEmpty()
	rm0.Resource().Attributes().PutStr("k8s.node.name", "node-1")
	sm0 := rm0.ScopeMetrics().AppendEmpty()
	sm0.Metrics().AppendEmpty().SetName("kube_pod_info")

	// RM 1: has node name with cache miss → should pass through
	rm1 := input.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("k8s.node.name", "node-unknown")
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	sm1.Metrics().AppendEmpty().SetName("kube_pod_info")

	// RM 2: no node name → should pass through (namespace-level metric)
	rm2 := input.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("k8s.namespace.name", "default")
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	sm2.Metrics().AppendEmpty().SetName("kube_namespace_status_phase")

	output, err := p.processMetrics(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 3, output.ResourceMetrics().Len())

	// RM 0: enriched
	attrs0 := output.ResourceMetrics().At(0).Resource().Attributes()
	hostID, ok := attrs0.Get(attrHostID)
	require.True(t, ok, "RM 0 should have host.id")
	assert.Equal(t, "i-0abc111def222", hostID.Str())

	// RM 1: not enriched (cache miss)
	attrs1 := output.ResourceMetrics().At(1).Resource().Attributes()
	_, ok = attrs1.Get(attrHostID)
	assert.False(t, ok, "RM 1 should NOT have host.id (cache miss)")

	// RM 2: not enriched (no node name)
	attrs2 := output.ResourceMetrics().At(2).Resource().Attributes()
	_, ok = attrs2.Get(attrHostID)
	assert.False(t, ok, "RM 2 should NOT have host.id (no node name)")
}
