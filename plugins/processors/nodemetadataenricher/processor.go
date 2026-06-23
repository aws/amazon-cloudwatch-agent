// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadataenricher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

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

	// missLogInterval rate-limits the cache-miss diagnostic so a fleet of
	// node-scoped KSM resources cannot flood the logs every scrape interval.
	missLogInterval = 1 * time.Minute
)

type nodeMetadataEnricherProcessor struct {
	logger *zap.Logger
	cache  atomic.Pointer[nodemetadatacache.NodeMetadataCache]

	// lastMissLogAt rate-limits the per-node cache-miss warning.
	missMu        sync.Mutex
	lastMissLogAt map[string]time.Time
}

func newNodeMetadataEnricherProcessor(logger *zap.Logger) *nodeMetadataEnricherProcessor {
	p := &nodeMetadataEnricherProcessor{
		logger:        logger,
		lastMissLogAt: make(map[string]time.Time),
	}
	if c := nodemetadatacache.GetNodeMetadataCache(); c != nil {
		p.cache.Store(c)
	}
	return p
}

func (p *nodeMetadataEnricherProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	cache := p.cache.Load()
	if cache == nil {
		// Extension may not have been ready at creation time — retry.
		cache = nodemetadatacache.GetNodeMetadataCache()
		if cache != nil {
			p.logger.Debug("Lazily initialized node metadata cache reference")
			p.cache.Store(cache)
		} else {
			return md, nil
		}
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		resource := rms.At(i).Resource()
		nodeNameVal, exists := resource.Attributes().Get(attrNodeName)
		if !exists || nodeNameVal.Str() == "" {
			continue
		}

		nodeName := nodeNameVal.Str()
		metadata := cache.Get(nodeName)
		if metadata == nil {
			// A resource carries k8s.node.name but the cache has no live Lease
			// for that name. This is the failure mode behind KSM node-scoped
			// metrics missing host.* attributes: either the node-name string
			// here differs from the Lease key (K8S_NODE_NAME) or no Lease has
			// been published/renewed for the node. Surface it (rate-limited)
			// with the cache contents so the discriminator is unambiguous.
			p.logCacheMiss(nodeName, cache)
			continue
		}

		resource.Attributes().PutStr(attrHostID, metadata.HostID)
		resource.Attributes().PutStr(attrHostName, metadata.HostName)
		resource.Attributes().PutStr(attrHostType, metadata.HostType)
		resource.Attributes().PutStr(attrHostImageID, metadata.HostImageID)
		resource.Attributes().PutStr(attrAvailabilityZone, metadata.AvailabilityZone)
	}

	return md, nil
}

// logCacheMiss emits a rate-limited warning (at most once per node name per
// missLogInterval) describing an enrichment miss and the current cache state.
func (p *nodeMetadataEnricherProcessor) logCacheMiss(nodeName string, cache *nodemetadatacache.NodeMetadataCache) {
	p.missMu.Lock()
	now := time.Now()
	if last, ok := p.lastMissLogAt[nodeName]; ok && now.Sub(last) < missLogInterval {
		p.missMu.Unlock()
		return
	}
	p.lastMissLogAt[nodeName] = now
	p.missMu.Unlock()

	p.logger.Warn("node metadata cache miss; host.* attributes not enriched for this resource",
		zap.String("k8s.node.name", nodeName),
		zap.Int("cacheSize", cache.Len()),
		zap.Strings("cachedNodeNames", cache.Keys()),
	)
}
