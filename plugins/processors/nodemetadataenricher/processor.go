// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadataenricher

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/nodemetadatacache"
)

const (
	attrNodeName         = "k8s.node.name"
	attrHostID           = "host.id"
	attrHostName         = "host.name"
	attrHostType         = "host.type"
	attrHostImageID      = "host.image.id"
	attrAvailabilityZone = "cloud.availability_zone"
)

type nodeMetadataEnricherProcessor struct {
	logger *zap.Logger
	cache  atomic.Pointer[nodemetadatacache.NodeMetadataCache]
}

func newNodeMetadataEnricherProcessor(logger *zap.Logger) *nodeMetadataEnricherProcessor {
	p := &nodeMetadataEnricherProcessor{
		logger: logger,
	}
	if c := nodemetadatacache.GetNodeMetadataCache(); c != nil {
		p.cache.Store(c)
	}
	return p
}

// resolveCache returns the node metadata cache, lazily initializing the
// processor's reference if the cache extension was not ready at construction
// time. Returns nil if the cache is still unavailable.
func (p *nodeMetadataEnricherProcessor) resolveCache() *nodemetadatacache.NodeMetadataCache {
	cache := p.cache.Load()
	if cache != nil {
		return cache
	}
	cache = nodemetadatacache.GetNodeMetadataCache()
	if cache != nil {
		p.logger.Debug("Lazily initialized node metadata cache reference")
		p.cache.Store(cache)
	}
	return cache
}

// enrichResource applies node metadata attributes to a single resource if it
// has a non-empty k8s.node.name attribute and a corresponding entry exists in
// the cache. Resources without a node name or with a cache miss are left
// untouched.
func (p *nodeMetadataEnricherProcessor) enrichResource(resource pcommon.Resource, cache *nodemetadatacache.NodeMetadataCache) {
	nodeNameVal, exists := resource.Attributes().Get(attrNodeName)
	if !exists || nodeNameVal.Str() == "" {
		return
	}

	metadata := cache.Get(nodeNameVal.Str())
	if metadata == nil {
		return
	}

	resource.Attributes().PutStr(attrHostID, metadata.HostID)
	resource.Attributes().PutStr(attrHostName, metadata.HostName)
	resource.Attributes().PutStr(attrHostType, metadata.HostType)
	resource.Attributes().PutStr(attrHostImageID, metadata.HostImageID)
	resource.Attributes().PutStr(attrAvailabilityZone, metadata.AvailabilityZone)
}

func (p *nodeMetadataEnricherProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	cache := p.resolveCache()
	if cache == nil {
		return md, nil
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		p.enrichResource(rms.At(i).Resource(), cache)
	}

	return md, nil
}

func (p *nodeMetadataEnricherProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	cache := p.resolveCache()
	if cache == nil {
		return ld, nil
	}

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		p.enrichResource(rls.At(i).Resource(), cache)
	}

	return ld, nil
}
