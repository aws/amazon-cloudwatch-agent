// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type Tagger struct {
	config   *Config
	logger   *zap.Logger
	provider DiskProvider

	done         chan struct{}
	shutdownOnce sync.Once
}

func newTagger(config *Config, logger *zap.Logger, provider DiskProvider) *Tagger {
	return &Tagger{
		config:   config,
		logger:   logger,
		provider: provider,
	}
}

func (t *Tagger) Start(_ context.Context, _ component.Host) error {
	if t.provider == nil {
		t.logger.Warn("disktagger: no provider, disk tagging disabled")
		return nil
	}

	if err := t.provider.Refresh(); err != nil {
		t.logger.Warn("Initial disk refresh failed, will retry", zap.Error(err))
	}

	if t.config.RefreshInterval > 0 {
		t.done = make(chan struct{})
		go t.refreshLoop()
	}
	return nil
}

func (t *Tagger) Shutdown(_ context.Context) error {
	if t.done != nil {
		t.shutdownOnce.Do(func() { close(t.done) })
	}
	return nil
}

func (t *Tagger) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if t.provider == nil {
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
	// Uses provider.Serial() which supports prefix matching
	// (e.g. metric device "nvme0n1p1" matches cached device "nvme0n1")
	serial := t.provider.Serial(devVal.Str())
	if serial != "" {
		attrs.PutStr(AttributeDiskID, serial)
	}
}

func (t *Tagger) refreshLoop() {
	ticker := time.NewTicker(t.config.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Refresh outside the lock (network I/O).
			if err := t.provider.Refresh(); err != nil {
				t.logger.Warn("Disk refresh failed", zap.Error(err))
			}
		case <-t.done:
			return
		}
	}
}
