// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsapplicationsignals

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	appsignalsconfig "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/cardinalitycontrol"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/normalizer"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/prune"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/resolver"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/rules"
)

const (
	failedToProcessAttribute            = "failed to process attributes"
	failedToProcessAttributeWithLimiter = "failed to process attributes with limiter, keep the data"
)

var metricCaser = cases.Title(language.English)

// this is used to Process some attributes (like IP addresses) to a generic form to reduce high cardinality
type attributesMutator interface {
	Process(attributes, resourceAttributes pcommon.Map, isTrace bool) error
}

type allowListMutator interface {
	ShouldBeDropped(attributes pcommon.Map) (bool, error)
}

type stopper interface {
	Stop(context.Context) error
}

type awsapplicationsignalsprocessor struct {
	logger            *zap.Logger
	config            *appsignalsconfig.Config
	replaceActions    *rules.ReplaceActions
	allowlistMutators []allowListMutator
	metricMutators    []attributesMutator
	traceMutators     []attributesMutator
	limiter           cardinalitycontrol.Limiter
	stoppers          []stopper
}

func (ap *awsapplicationsignalsprocessor) StartMetrics(ctx context.Context, _ component.Host) error {
	attributesResolver := resolver.NewAttributesResolver(ap.config.Resolvers, ap.logger)
	ap.stoppers = []stopper{attributesResolver}
	attributesNormalizer := normalizer.NewAttributesNormalizer(ap.logger)
	ap.metricMutators = []attributesMutator{attributesResolver, attributesNormalizer}

	limiterConfig := ap.config.Limiter
	if limiterConfig == nil {
		limiterConfig = appsignalsconfig.NewDefaultLimiterConfig()
	}
	if limiterConfig.ParentContext == nil {
		limiterConfig.ParentContext = ctx
	}

	if !limiterConfig.Disabled {
		ap.limiter = cardinalitycontrol.NewMetricsLimiter(limiterConfig, ap.logger)
	} else {
		ap.logger.Info("metrics limiter is disabled.")
	}

	ap.replaceActions = rules.NewReplacer(ap.config.Rules, !limiterConfig.Disabled)

	pruner := prune.NewPruner()
	keeper := rules.NewKeeper(ap.config.Rules, !limiterConfig.Disabled)
	dropper := rules.NewDropper(ap.config.Rules)
	ap.allowlistMutators = []allowListMutator{pruner, keeper, dropper}

	return nil
}

func (ap *awsapplicationsignalsprocessor) StartTraces(_ context.Context, _ component.Host) error {
	attributesResolver := resolver.NewAttributesResolver(ap.config.Resolvers, ap.logger)
	attributesNormalizer := normalizer.NewAttributesNormalizer(ap.logger)
	customReplacer := rules.NewReplacer(ap.config.Rules, false)

	ap.stoppers = append(ap.stoppers, attributesResolver)
	ap.traceMutators = append(ap.traceMutators, attributesResolver, attributesNormalizer, customReplacer)
	return nil
}

func (ap *awsapplicationsignalsprocessor) Shutdown(ctx context.Context) error {
	for _, stopper := range ap.stoppers {
		err := stopper.Stop(ctx)
		if err != nil {
			ap.logger.Error("failed to stop", zap.Error(err))
		}
	}
	return nil
}

func (ap *awsapplicationsignalsprocessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
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

func (ap *awsapplicationsignalsprocessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
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
				m.SetName(metricCaser.String(m.Name())) // Ensure metric name is in sentence case
				ap.processMetricAttributes(ctx, m, resourceAttributes)
			}
		}
	}
	return md, nil
}

// Attributes are provided for each log and trace, but not at the metric level
// Need to process attributes for every data point within a metric.
func (ap *awsapplicationsignalsprocessor) processMetricAttributes(_ context.Context, m pmetric.Metric, resourceAttribes pcommon.Map) {
	// This is a lot of repeated code, but since there is no single parent superclass
	// between metric data types, we can't use polymorphism.
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, mutator := range ap.metricMutators {
				err := mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.NumberDataPoint) bool {
			for _, mutator := range ap.allowlistMutators {
				shouldBeDropped, err := mutator.ShouldBeDropped(d.Attributes())
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
				if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.replaceActions.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
		if ap.limiter != nil {
			for i := 0; i < dps.Len(); i++ {
				if _, err := ap.limiter.Admit(m.Name(), dps.At(i).Attributes(), resourceAttribes); err != nil {
					ap.logger.Debug(failedToProcessAttributeWithLimiter, zap.Error(err))
				}
			}
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, mutator := range ap.metricMutators {
				err := mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.NumberDataPoint) bool {
			for _, mutator := range ap.allowlistMutators {
				shouldBeDropped, err := mutator.ShouldBeDropped(d.Attributes())
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
				if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.replaceActions.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
		if ap.limiter != nil {
			for i := 0; i < dps.Len(); i++ {
				if _, err := ap.limiter.Admit(m.Name(), dps.At(i).Attributes(), resourceAttribes); err != nil {
					ap.logger.Debug(failedToProcessAttributeWithLimiter, zap.Error(err))
				}
			}
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, mutator := range ap.metricMutators {
				err := mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.HistogramDataPoint) bool {
			for _, mutator := range ap.allowlistMutators {
				shouldBeDropped, err := mutator.ShouldBeDropped(d.Attributes())
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
				if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.replaceActions.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
		if ap.limiter != nil {
			for i := 0; i < dps.Len(); i++ {
				if _, err := ap.limiter.Admit(m.Name(), dps.At(i).Attributes(), resourceAttribes); err != nil {
					ap.logger.Debug(failedToProcessAttributeWithLimiter, zap.Error(err))
				}
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, mutator := range ap.metricMutators {
				err := mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.ExponentialHistogramDataPoint) bool {
			for _, mutator := range ap.allowlistMutators {
				shouldBeDropped, err := mutator.ShouldBeDropped(d.Attributes())
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
				if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.replaceActions.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
		if ap.limiter != nil {
			for i := 0; i < dps.Len(); i++ {
				if _, err := ap.limiter.Admit(m.Name(), dps.At(i).Attributes(), resourceAttribes); err != nil {
					ap.logger.Debug(failedToProcessAttributeWithLimiter, zap.Error(err))
				}
			}
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			for _, mutator := range ap.metricMutators {
				err := mutator.Process(dps.At(i).Attributes(), resourceAttribes, false)
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
			}
		}
		dps.RemoveIf(func(d pmetric.SummaryDataPoint) bool {
			for _, mutator := range ap.allowlistMutators {
				shouldBeDropped, err := mutator.ShouldBeDropped(d.Attributes())
				if err != nil {
					ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
				}
				if shouldBeDropped {
					return true
				}
			}
			return false
		})
		for i := 0; i < dps.Len(); i++ {
			err := ap.replaceActions.Process(dps.At(i).Attributes(), resourceAttribes, false)
			if err != nil {
				ap.logger.Debug(failedToProcessAttribute, zap.Error(err))
			}
		}
		if ap.limiter != nil {
			for i := 0; i < dps.Len(); i++ {
				if _, err := ap.limiter.Admit(m.Name(), dps.At(i).Attributes(), resourceAttribes); err != nil {
					ap.logger.Debug(failedToProcessAttributeWithLimiter, zap.Error(err))
				}
			}
		}
	default:
		ap.logger.Debug("Ignore unknown metric type", zap.String("type", m.Type().String()))
	}
}
