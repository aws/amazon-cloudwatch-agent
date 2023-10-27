// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsappsignals

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/customconfiguration"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/internal/normalizer"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/internal/resolver"
)

const (
	failedToProcessAttribute               = "failed to process attributes"
	failedToProcessAttributeWithCustomRule = "failed to process attributes with custom rule, will drop the metric"
)

// this is used to Process some attributes (like IP addresses) to a generic form to reduce high cardinality
type attributesMutator interface {
	Process(attributes, resourceAttributes pcommon.Map, isTrace bool) error
}

type customAllowlistMutator interface {
	ShouldBeDropped(attributes, resourceAttributes pcommon.Map) (bool, error)
}

type stopper interface {
	Stop(context.Context) error
}

type awsappsignalsprocessor struct {
	logger                  *zap.Logger
	config                  *Config
	customReplacer          *customconfiguration.ReplaceActions
	customAllowlistMutators []customAllowlistMutator
	metricMutators          []attributesMutator
	traceMutators           []attributesMutator
	stoppers                []stopper
}

func (ap *awsappsignalsprocessor) Start(_ context.Context, _ component.Host) error {
	attributesResolver := resolver.NewAttributesResolver(ap.config.Resolvers, ap.logger)
	ap.stoppers = []stopper{attributesResolver}
	ap.metricMutators = []attributesMutator{attributesResolver}

	attributesNormalizer := normalizer.NewAttributesNormalizer(ap.logger)
	ap.metricMutators = []attributesMutator{attributesResolver, attributesNormalizer}

	ap.customReplacer = customconfiguration.NewCustomReplacer(ap.config.Rules)
	ap.traceMutators = []attributesMutator{attributesResolver, attributesNormalizer, ap.customReplacer}

	customKeeper := customconfiguration.NewCustomKeeper(ap.config.Rules)
	ap.customAllowlistMutators = []customAllowlistMutator{customKeeper}

	customDropper := customconfiguration.NewCustomDropper(ap.config.Rules)
	ap.customAllowlistMutators = []customAllowlistMutator{customDropper}

	return nil
}

func (ap *awsappsignalsprocessor) Shutdown(ctx context.Context) error {
	for _, stopper := range ap.stoppers {
		err := stopper.Stop(ctx)
		if err != nil {
			ap.logger.Error("failed to stop", zap.Error(err))
		}
	}
	return nil
}

func (ap *awsappsignalsprocessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		resourceAttributes := rs.Resource().Attributes()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				for _, Mutator := range ap.traceMutators {
					err := Mutator.Process(span.Attributes(), resourceAttributes, true)
					if err != nil {
						ap.logger.Debug("failed to Process span", zap.Error(err))
					}
				}
			}
		}
	}
	return td, nil
}

func (ap *awsappsignalsprocessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		resourceAttributes := rs.Resource().Attributes()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				ap.processMetricAttributes(ctx, m, resourceAttributes)
			}
		}
	}
	return md, nil
}

// Attributes are provided for each log and trace, but not at the metric level
// Need to process attributes for every data point within a metric.
func (ap *awsappsignalsprocessor) processMetricAttributes(ctx context.Context, m pmetric.Metric, resourceAttribes pcommon.Map) {

	// This is a lot of repeated code, but since there is no single parent superclass
	// between metric data types, we can't use polymorphism.
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, Mutator := range ap.metricMutators {
				err := Mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.NumberDataPoint) bool {
			for _, Mutator := range ap.customAllowlistMutators {
				shouldBeDropped, err := Mutator.ShouldBeDropped(d.Attributes(), resourceAttribes)
				if err != nil {
					ap.logger.Debug(failedToProcessAttributeWithCustomRule, zap.Error(err))
					return true
				} else if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.customReplacer.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, Mutator := range ap.metricMutators {
				err := Mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.NumberDataPoint) bool {
			for _, Mutator := range ap.customAllowlistMutators {
				shouldBeDropped, err := Mutator.ShouldBeDropped(d.Attributes(), resourceAttribes)
				if err != nil {
					ap.logger.Debug(failedToProcessAttributeWithCustomRule, zap.Error(err))
					return true
				} else if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.customReplacer.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, Mutator := range ap.metricMutators {
				err := Mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.HistogramDataPoint) bool {
			for _, Mutator := range ap.customAllowlistMutators {
				shouldBeDropped, err := Mutator.ShouldBeDropped(d.Attributes(), resourceAttribes)
				if err != nil {
					ap.logger.Debug(failedToProcessAttributeWithCustomRule, zap.Error(err))
					return true
				} else if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.customReplacer.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, Mutator := range ap.metricMutators {
				err := Mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.ExponentialHistogramDataPoint) bool {
			for _, Mutator := range ap.customAllowlistMutators {
				shouldBeDropped, err := Mutator.ShouldBeDropped(d.Attributes(), resourceAttribes)
				if err != nil {
					ap.logger.Debug(failedToProcessAttributeWithCustomRule, zap.Error(err))
					return true
				} else if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.customReplacer.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, Mutator := range ap.metricMutators {
				err := Mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.SummaryDataPoint) bool {
			for _, Mutator := range ap.customAllowlistMutators {
				shouldBeDropped, err := Mutator.ShouldBeDropped(d.Attributes(), resourceAttribes)
				if err != nil {
					ap.logger.Debug(failedToProcessAttributeWithCustomRule, zap.Error(err))
					return true
				} else if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.customReplacer.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
	default:
		ap.logger.Debug("Ignore unknown metric type", zap.String("type", m.Type().String()))
	}
}
