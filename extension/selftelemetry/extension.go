// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package selftelemetry publishes the agent's prometheus scrape and discovery registries on the
// collector's own service::telemetry endpoint. The collector builds that endpoint's registry
// privately, so the only way onto it is to record through the MeterProvider every component is
// given, which is what this extension bridges into.
package selftelemetry

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	scopeName = "github.com/aws/amazon-cloudwatch-agent/extension/selftelemetry"

	// familyPrefix limits the bridge to the discovery and scrape families. The registries also hold
	// go_ and process_ collectors, which the collector already reports as otelcol_process_ metrics.
	familyPrefix = "prometheus_"

	receiverLabel = "receiver"
	// telegrafReceiver identifies the Telegraf prometheus plugin, whose series carry no receiver
	// label of their own, so every published series still names where it came from.
	telegrafReceiver = "telegraf/prometheus"
)

// source pairs a registry with the receiver label to apply when its series lack one.
type source struct {
	gatherer prometheus.Gatherer
	fallback string
}

type selfTelemetry struct {
	logger *zap.Logger
	cfg    *Config
	meter  otelmetric.Meter

	cancel context.CancelFunc

	// sources is a field so tests can supply registries instead of the process-wide ones.
	sources func() []source

	// instruments and registration are only touched by the sync goroutine; each callback closes over
	// its own snapshot, so the observe path needs no locking.
	instruments  map[string]otelmetric.Float64Observable
	registration otelmetric.Registration
	shutdownOnce sync.Once
}

var _ extension.Extension = (*selfTelemetry)(nil)

func newSelfTelemetry(settings extension.Settings, cfg *Config) *selfTelemetry {
	return &selfTelemetry{
		logger:      settings.Logger,
		cfg:         cfg,
		meter:       settings.MeterProvider.Meter(scopeName),
		sources:     processSources,
		instruments: map[string]otelmetric.Float64Observable{},
	}
}

// processSources is resolved on every pass so receivers that start or stop later are picked up.
// SharedGatherer already labels each series with its receiver; the Telegraf plugin's registry does
// not, so it gets a fallback label.
func processSources() []source {
	return []source{
		{gatherer: prometheusreceiver.SharedGatherer()},
		{gatherer: prometheus.DefaultGatherer, fallback: telegrafReceiver},
	}
}

func (s *selfTelemetry) Start(_ context.Context, _ component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.run(ctx)
	return nil
}

// run rescans on an interval because extensions start before receivers, so nothing is registered yet
// on the first pass, and families keep appearing as counters are first incremented.
func (s *selfTelemetry) run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.discoveryInterval())
	defer ticker.Stop()
	s.sync()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sync()
		}
	}
}

func (s *selfTelemetry) sync() {
	var added bool
	for _, src := range s.sources() {
		families, err := src.gatherer.Gather()
		if err != nil {
			// Gather returns partial results alongside errors, so keep the families it did return.
			s.logger.Debug("partial self-telemetry gather", zap.Error(err))
		}
		for _, mf := range families {
			if !strings.HasPrefix(mf.GetName(), familyPrefix) {
				continue
			}
			if _, exists := s.instruments[mf.GetName()]; exists {
				continue
			}
			inst, err := s.newInstrument(mf)
			if err != nil {
				s.logger.Warn("unable to publish self-telemetry family",
					zap.String("name", mf.GetName()), zap.Error(err))
				continue
			}
			if inst == nil {
				continue
			}
			s.instruments[mf.GetName()] = inst
			added = true
		}
	}
	if added {
		s.reregister()
	}
}

// newInstrument returns nil for histograms and summaries, which have no observable equivalent.
func (s *selfTelemetry) newInstrument(mf *dto.MetricFamily) (otelmetric.Float64Observable, error) {
	desc := otelmetric.WithDescription(mf.GetHelp())
	switch mf.GetType() {
	case dto.MetricType_GAUGE, dto.MetricType_UNTYPED:
		return s.meter.Float64ObservableGauge(mf.GetName(), desc)
	case dto.MetricType_COUNTER:
		return s.meter.Float64ObservableCounter(mf.GetName(), desc)
	default:
		return nil, nil
	}
}

// reregister swaps in a single callback covering every instrument, so one gather serves a collection
// no matter how many passes it took to discover them.
func (s *selfTelemetry) reregister() {
	snapshot := make(map[string]otelmetric.Float64Observable, len(s.instruments))
	observables := make([]otelmetric.Observable, 0, len(s.instruments))
	for name, inst := range s.instruments {
		snapshot[name] = inst
		observables = append(observables, inst)
	}
	if s.registration != nil {
		if err := s.registration.Unregister(); err != nil {
			s.logger.Warn("unable to replace self-telemetry callback", zap.Error(err))
			return
		}
		s.registration = nil
	}
	registration, err := s.meter.RegisterCallback(s.observe(snapshot), observables...)
	if err != nil {
		s.logger.Error("unable to register self-telemetry callback", zap.Error(err))
		return
	}
	s.registration = registration
	s.logger.Debug("publishing self-telemetry families", zap.Int("count", len(observables)))
}

func (s *selfTelemetry) observe(snapshot map[string]otelmetric.Float64Observable) otelmetric.Callback {
	return func(_ context.Context, observer otelmetric.Observer) error {
		for _, src := range s.sources() {
			families, err := src.gatherer.Gather()
			if err != nil {
				s.logger.Debug("partial self-telemetry gather", zap.Error(err))
			}
			for _, mf := range families {
				inst, ok := snapshot[mf.GetName()]
				if !ok {
					continue
				}
				for _, m := range mf.GetMetric() {
					value, ok := valueOf(mf.GetType(), m)
					if !ok {
						continue
					}
					observer.ObserveFloat64(inst, value,
						otelmetric.WithAttributes(seriesAttributes(m, src.fallback)...))
				}
			}
		}
		return nil
	}
}

func valueOf(t dto.MetricType, m *dto.Metric) (float64, bool) {
	switch t {
	case dto.MetricType_GAUGE:
		return m.GetGauge().GetValue(), true
	case dto.MetricType_UNTYPED:
		return m.GetUntyped().GetValue(), true
	case dto.MetricType_COUNTER:
		return m.GetCounter().GetValue(), true
	default:
		return 0, false
	}
}

// seriesAttributes copies a scraped series' own prometheus labels onto its OTel datapoint, applying
// the receiver fallback when the source registry did not label the series itself (the Telegraf case).
func seriesAttributes(m *dto.Metric, fallback string) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(m.GetLabel())+1)
	var hasReceiver bool
	for _, l := range m.GetLabel() {
		if l.GetName() == receiverLabel {
			hasReceiver = true
		}
		attrs = append(attrs, attribute.String(l.GetName(), l.GetValue()))
	}
	if !hasReceiver && fallback != "" {
		attrs = append(attrs, attribute.String(receiverLabel, fallback))
	}
	return attrs
}

func (s *selfTelemetry) Shutdown(_ context.Context) error {
	s.shutdownOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})
	return nil
}
