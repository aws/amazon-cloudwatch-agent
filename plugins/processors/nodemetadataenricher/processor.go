// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadataenricher

import (
	"context"
	"sync/atomic"

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

func (p *nodeMetadataEnricherProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	cache := p.cache.Load()
	if cache == nil {
		// Extension may not have been ready at creation time — retry.
		cache = nodemetadatacache.GetNodeMetadataCache()
		if cache != nil {
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

		metadata := cache.Get(nodeNameVal.Str())
		if metadata == nil {
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
