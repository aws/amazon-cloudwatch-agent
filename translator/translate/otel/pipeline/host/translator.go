// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/prometheusremotewrite"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/deltatocumulativeprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

type translator struct {
	name string
	common.DestinationProvider
	receivers common.ComponentTranslatorMap
}

var _ common.PipelineTranslator = (*translator)(nil)

var supportedEntityProcessorDestinations = [...]string{
	common.DefaultDestination,
	common.CloudWatchKey,
	common.CloudWatchLogsKey,
}

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(
	name string,
	receivers common.ComponentTranslatorMap,
	opts ...common.TranslatorOption,
) common.PipelineTranslator {
	t := &translator{name: name, receivers: receivers}
	for _, opt := range opts {
		opt(t)
	}
	if t.Destination() != "" {
		t.name += "/" + t.Destination()
	}
	return t
}

func (t translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, t.name)
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || t.receivers.Len() == 0 {
		return nil, fmt.Errorf("no receivers configured in pipeline %s", t.name)
	}

	var entityProcessor common.ComponentTranslator
	var ec2TaggerEnabled bool

	translators := common.ComponentTranslators{
		Receivers:  t.receivers,
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	if strings.HasPrefix(t.name, common.PipelineNameHostDeltaMetrics) || strings.HasPrefix(t.name, common.PipelineNameHostOtlpMetrics) {
		log.Printf("D! delta processor required because metrics with diskio or net are set")
		translators.Processors.Set(cumulativetodeltaprocessor.NewTranslator(common.WithName(t.name), cumulativetodeltaprocessor.WithDefaultKeys()))
	}

	if t.Destination() != common.CloudWatchLogsKey {
		if conf.IsSet(common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)) {
			log.Printf("D! ec2tagger processor required because append_dimensions is set")
			translators.Processors.Set(ec2taggerprocessor.NewTranslator())
			ec2TaggerEnabled = true
		}

		mdt := metricsdecorator.NewTranslator(metricsdecorator.WithIgnorePlugins(common.JmxKey))
		if mdt.IsSet(conf) {
			log.Printf("D! metric decorator required because measurement fields are set")
			translators.Processors.Set(mdt)
		}
	}

	currentContext := context.CurrentContext()

	switch determinePipeline(t.name) {
	case common.PipelineNameHostOtlpMetrics:
		// TODO: For OTLP, the entity processor is only on K8S for now. Eventually this should be added to EC2
		if currentContext.KubernetesMode() != "" {
			entityProcessor = awsentity.NewTranslatorWithEntityType(awsentity.Service, common.OtlpKey, false)
		}
	case common.PipelineNameHostCustomMetrics:
		if !currentContext.RunInContainer() {
			entityProcessor = awsentity.NewTranslatorWithEntityType(awsentity.Service, "telegraf", true)
		}
	case common.PipelineNameHost, common.PipelineNameHostDeltaMetrics:
		if !currentContext.RunInContainer() {
			entityProcessor = awsentity.NewTranslatorWithEntityType(awsentity.Resource, "", ec2TaggerEnabled)
		}
	}

	validDestination := slices.Contains(supportedEntityProcessorDestinations[:], t.Destination())
	// ECS is not in scope for entity association, so we only add the entity processor in non-ECS platforms
	isECS := ecsutil.GetECSUtilSingleton().IsECS()
	if entityProcessor != nil && currentContext.Mode() == config.ModeEC2 && !isECS && validDestination {
		translators.Processors.Set(entityProcessor)
	}

	switch t.Destination() {
	case common.DefaultDestination, common.CloudWatchKey:
		translators.Exporters.Set(awscloudwatch.NewTranslator())
		translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.MetricsName, []string{agenthealth.OperationPutMetricData}))
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))
	case common.AMPKey:
		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			translators.Processors.Set(rollupprocessor.NewTranslator())
		}
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey))
		translators.Processors.Set(deltatocumulativeprocessor.NewTranslator(common.WithName(t.name)))
		translators.Exporters.Set(prometheusremotewrite.NewTranslatorWithName(common.AMPKey))
		translators.Extensions.Set(sigv4auth.NewTranslator())
	case common.CloudWatchLogsKey:
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.LogsKey))
		translators.Exporters.Set(awsemf.NewTranslator())
		translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))
	default:
		return nil, fmt.Errorf("pipeline (%s) does not support destination (%s) in configuration", t.name, t.Destination())
	}

	return &translators, nil
}

func determinePipeline(name string) string {
	// The conditionals have to be done in a certain order because PipelineNameHost is just "host", whereas
	// the other constants are prefixed with "host"
	if strings.HasPrefix(name, common.PipelineNameHostDeltaMetrics) {
		return common.PipelineNameHostDeltaMetrics
	} else if strings.HasPrefix(name, common.PipelineNameHostOtlpMetrics) {
		return common.PipelineNameHostOtlpMetrics
	} else if strings.HasPrefix(name, common.PipelineNameHostCustomMetrics) {
		return common.PipelineNameHostCustomMetrics
	} else if strings.HasPrefix(name, common.PipelineNameHost) {
		return common.PipelineNameHost
	}
	return ""
}
