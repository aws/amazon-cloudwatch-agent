// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/disktagger/internal/volume"
)

type Tagger struct {
	config       *Config
	logger       *zap.Logger
	cacheFactory cacheFactory
	cache        volume.Cache
	cancel       context.CancelFunc
}

func newTagger(config *Config, logger *zap.Logger, factory cacheFactory) *Tagger {
	return &Tagger{
		config:       config,
		logger:       logger,
		cacheFactory: factory,
	}
}

func (t *Tagger) Start(ctx context.Context, _ component.Host) error {
	t.cache = t.cacheFactory(t.config)
	if t.cache == nil {
		t.logger.Warn("disktagger: no provider configured, disk tagging disabled")
		return nil
	}

	if err := t.cache.Refresh(ctx); err != nil {
		t.logger.Warn("Initial disk refresh failed, will retry", zap.Error(err))
	}

	if t.config.RefreshInterval > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		t.cancel = cancel
		go t.refreshLoop(ctx)
	}
	return nil
}

func (t *Tagger) Shutdown(_ context.Context) error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

func (t *Tagger) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if t.cache == nil {
		return md, nil
	}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				t.tagMetric(sm.Metrics().At(k))
			}
		}
	}
	return md, nil
}

func (t *Tagger) tagMetric(m pmetric.Metric) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
			t.tagDataPoint(m.Gauge().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < m.Sum().DataPoints().Len(); i++ {
			t.tagDataPoint(m.Sum().DataPoints().At(i).Attributes())
		}
	}
}

func (t *Tagger) tagDataPoint(attrs pcommon.Map) {
	if _, exists := attrs.Get(AttributeDiskID); exists {
		return
	}
	devVal, found := attrs.Get(t.config.DiskDeviceTagKey)
	if !found {
		return
	}
	serial := t.cache.Serial(devVal.Str())
	if serial != "" {
		attrs.PutStr(AttributeDiskID, serial)
	}
}

func (t *Tagger) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(t.config.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := t.cache.Refresh(ctx); err != nil {
				t.logger.Warn("Disk refresh failed", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}
