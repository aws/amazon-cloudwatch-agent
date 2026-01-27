// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuretagger

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
)

// azureMetadataLookupType tracks which metadata fields to include
type azureMetadataLookupType struct {
	instanceID        bool
	imageID           bool
	instanceType      bool
	vmScaleSetName    bool
	resourceGroupName bool
	subscriptionID    bool
}

// azureMetadataRespondType caches metadata values
type azureMetadataRespondType struct {
	instanceID        string
	imageID           string
	instanceType      string
	vmScaleSetName    string
	resourceGroupName string
	subscriptionID    string
	region            string
}

// Tagger is the Azure tagger processor
type Tagger struct {
	*Config

	logger     *zap.Logger
	cancelFunc context.CancelFunc

	shutdownC            chan bool
	azureTagCache        map[string]string
	started              bool
	azureMetadataLookup  azureMetadataLookupType
	azureMetadataRespond azureMetadataRespondType
	useAllTags           bool

	sync.RWMutex
}

// newTagger creates a new Azure Tagger processor
func newTagger(config *Config, logger *zap.Logger) *Tagger {
	_, cancel := context.WithCancel(context.Background())
	return &Tagger{
		Config:     config,
		logger:     logger,
		cancelFunc: cancel,
	}
}

// Start initializes the Azure tagger processor
func (t *Tagger) Start(_ context.Context, _ component.Host) error {
	t.shutdownC = make(chan bool)
	t.azureTagCache = map[string]string{}

	// Get CMCA provider
	provider := cloudmetadata.GetGlobalProviderOrNil()
	if provider == nil {
		t.logger.Info("azuretagger: Cloud metadata provider not available, processor disabled")
		t.setStarted()
		return nil
	}

	// Check if we're on Azure
	if provider.GetCloudProvider() != int(cloudmetadata.CloudProviderAzure) {
		t.logger.Info("azuretagger: Not running on Azure, processor disabled",
			zap.Int("cloudProvider", provider.GetCloudProvider()))
		t.setStarted()
		return nil
	}

	// Derive metadata from CMCA provider
	if err := t.deriveAzureMetadataFromProvider(provider); err != nil {
		t.logger.Warn("azuretagger: Failed to derive Azure metadata", zap.Error(err))
		// Continue anyway - graceful degradation
	}

	// Fetch initial tags
	t.useAllTags = len(t.AzureInstanceTagKeys) == 1 && t.AzureInstanceTagKeys[0] == "*"
	if len(t.AzureInstanceTagKeys) > 0 {
		t.updateTagsFromProvider(provider)
	}

	// Start refresh loop if configured
	if t.RefreshTagsInterval > 0 && len(t.AzureInstanceTagKeys) > 0 {
		go t.refreshLoopTags(provider)
	}

	t.setStarted()
	t.logger.Info("azuretagger: Azure tagger started",
		zap.Int("tagCount", len(t.azureTagCache)),
		zap.Duration("refreshInterval", t.RefreshTagsInterval))

	return nil
}

// deriveAzureMetadataFromProvider extracts metadata from the CMCA provider
func (t *Tagger) deriveAzureMetadataFromProvider(provider cloudmetadata.Provider) error {
	// Parse which metadata tags to include
	for _, tag := range t.AzureMetadataTags {
		switch tag {
		case MdKeyInstanceID:
			t.azureMetadataLookup.instanceID = true
		case MdKeyImageID:
			t.azureMetadataLookup.imageID = true
		case MdKeyInstanceType:
			t.azureMetadataLookup.instanceType = true
		case MdKeyVMScaleSetName:
			t.azureMetadataLookup.vmScaleSetName = true
		case MdKeyResourceGroupName:
			t.azureMetadataLookup.resourceGroupName = true
		case MdKeySubscriptionID:
			t.azureMetadataLookup.subscriptionID = true
		default:
			t.logger.Warn("azuretagger: Unsupported Azure metadata key", zap.String("key", tag))
		}
	}

	// Fetch values from provider
	t.azureMetadataRespond.region = provider.GetRegion()
	t.azureMetadataRespond.instanceID = provider.GetInstanceID()

	if t.azureMetadataLookup.imageID {
		t.azureMetadataRespond.imageID = provider.GetImageID()
	}
	if t.azureMetadataLookup.instanceType {
		t.azureMetadataRespond.instanceType = provider.GetInstanceType()
	}
	if t.azureMetadataLookup.vmScaleSetName {
		t.azureMetadataRespond.vmScaleSetName = provider.GetScalingGroupName()
	}
	if t.azureMetadataLookup.resourceGroupName {
		t.azureMetadataRespond.resourceGroupName = provider.GetResourceGroupName()
	}
	if t.azureMetadataLookup.subscriptionID {
		t.azureMetadataRespond.subscriptionID = provider.GetAccountID()
	}

	t.logger.Debug("azuretagger: Azure metadata derived",
		zap.String("region", t.azureMetadataRespond.region),
		zap.String("instanceID", maskValue(t.azureMetadataRespond.instanceID)))

	return nil
}

// updateTagsFromProvider fetches tags from the CMCA provider
func (t *Tagger) updateTagsFromProvider(provider cloudmetadata.Provider) {
	tags := provider.GetTags()

	t.Lock()
	defer t.Unlock()

	if t.useAllTags {
		// Use all tags
		t.azureTagCache = tags
	} else {
		// Filter to requested tags only
		t.azureTagCache = make(map[string]string)
		for _, key := range t.AzureInstanceTagKeys {
			if val, ok := tags[key]; ok {
				t.azureTagCache[key] = val
			}
		}
	}

	t.logger.Debug("azuretagger: Tags updated",
		zap.Int("tagCount", len(t.azureTagCache)))
}

// refreshLoopTags periodically refreshes tags from IMDS
func (t *Tagger) refreshLoopTags(provider cloudmetadata.Provider) {
	refreshInterval := t.RefreshTagsInterval
	if refreshInterval <= 0 {
		refreshInterval = defaultRefreshInterval
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.logger.Debug("azuretagger: Refreshing tags")

			// Refresh the provider's metadata (which includes tags)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := provider.Refresh(ctx); err != nil {
				t.logger.Warn("azuretagger: Failed to refresh Azure metadata", zap.Error(err))
				cancel()
				continue
			}
			cancel()

			// Update our tag cache
			t.updateTagsFromProvider(provider)

		case <-t.shutdownC:
			t.logger.Debug("azuretagger: Refresh loop stopped")
			return
		}
	}
}

// processMetrics adds Azure tags and metadata to metrics
func (t *Tagger) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	t.RLock()
	defer t.RUnlock()

	if !t.started {
		return pmetric.NewMetrics(), nil
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				attributes := getOtelAttributes(metrics.At(k))
				t.updateOtelAttributes(attributes)
			}
		}
	}
	return md, nil
}

// getOtelAttributes extracts attributes from all data points in a metric
func getOtelAttributes(m pmetric.Metric) []pcommon.Map {
	attributes := []pcommon.Map{}
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			attributes = append(attributes, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			attributes = append(attributes, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			attributes = append(attributes, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			attributes = append(attributes, dps.At(i).Attributes())
		}
	}
	return attributes
}

// updateOtelAttributes adds Azure tags and metadata to metric attributes
func (t *Tagger) updateOtelAttributes(attributes []pcommon.Map) {
	for _, attr := range attributes {
		// Add Azure tags
		if t.azureTagCache != nil {
			for k, v := range t.azureTagCache {
				if _, exists := attr.Get(k); !exists {
					attr.PutStr(k, v)
				}
			}
		}

		// Add Azure metadata dimensions
		if t.azureMetadataLookup.instanceID {
			if _, exists := attr.Get(MdKeyInstanceID); !exists {
				attr.PutStr(MdKeyInstanceID, t.azureMetadataRespond.instanceID)
			}
		}
		if t.azureMetadataLookup.imageID {
			if _, exists := attr.Get(MdKeyImageID); !exists {
				attr.PutStr(MdKeyImageID, t.azureMetadataRespond.imageID)
			}
		}
		if t.azureMetadataLookup.instanceType {
			if _, exists := attr.Get(MdKeyInstanceType); !exists {
				attr.PutStr(MdKeyInstanceType, t.azureMetadataRespond.instanceType)
			}
		}
		if t.azureMetadataLookup.vmScaleSetName {
			if _, exists := attr.Get(MdKeyVMScaleSetName); !exists && t.azureMetadataRespond.vmScaleSetName != "" {
				attr.PutStr(MdKeyVMScaleSetName, t.azureMetadataRespond.vmScaleSetName)
			}
		}
		if t.azureMetadataLookup.resourceGroupName {
			if _, exists := attr.Get(MdKeyResourceGroupName); !exists {
				attr.PutStr(MdKeyResourceGroupName, t.azureMetadataRespond.resourceGroupName)
			}
		}
		if t.azureMetadataLookup.subscriptionID {
			if _, exists := attr.Get(MdKeySubscriptionID); !exists {
				attr.PutStr(MdKeySubscriptionID, t.azureMetadataRespond.subscriptionID)
			}
		}

		// Remove host attribute (same as ec2tagger)
		attr.Remove("host")
	}
}

// Shutdown stops the Azure tagger processor
func (t *Tagger) Shutdown(_ context.Context) error {
	if t.shutdownC != nil {
		close(t.shutdownC)
	}
	t.cancelFunc()
	return nil
}

// setStarted marks the processor as started
func (t *Tagger) setStarted() {
	t.Lock()
	t.started = true
	t.Unlock()
}

// maskValue masks sensitive values for logging
func maskValue(value string) string {
	if value == "" {
		return "<empty>"
	}
	if len(value) <= 4 {
		return "<present>"
	}
	return value[:4] + "..."
}
