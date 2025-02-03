// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/debug"
	"log"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

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
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/k8sattributesprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricsdecorator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/rollupprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

type translator struct {
	name string
	common.DestinationProvider
	receivers common.TranslatorMap[component.Config]
}

var supportedEntityProcessorDestinations = [...]string{
	common.DefaultDestination,
	common.CloudWatchKey,
	common.CloudWatchLogsKey,
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

// NewTranslator creates a new host pipeline translator. The receiver types
// passed in are converted to config.ComponentIDs, sorted, and used directly
// in the translated pipeline.
func NewTranslator(
	name string,
	receivers common.TranslatorMap[component.Config],
	opts ...common.TranslatorOption,
) common.Translator[*common.ComponentTranslators] {
	t := &translator{name: name, receivers: receivers}
	for _, opt := range opts {
		opt(t)
	}
	if t.Destination() != "" {
		t.name += "/" + t.Destination()
	}
	return t
}

func (t translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, t.name)
}

// Translate creates a pipeline if metrics section exists.
func (t translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || t.receivers.Len() == 0 {
		return nil, fmt.Errorf("no receivers configured in pipeline %s", t.name)
	}

	var entityProcessor common.Translator[component.Config]
	var ec2TaggerEnabled bool

	translators := common.ComponentTranslators{
		Receivers:  t.receivers,
		Processors: common.NewTranslatorMap[component.Config](),
		Exporters:  common.NewTranslatorMap[component.Config](),
		Extensions: common.NewTranslatorMap[component.Config](),
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
			translators.Processors.Set(k8sattributesprocessor.NewTranslatorWithName(t.name))
			entityProcessor = awsentity.NewTranslatorWithEntityType(awsentity.Service, common.OtlpKey, false)
			if enabled, _ := common.GetBool(conf, common.AgentDebugConfigKey); enabled {
				translators.Exporters.Set(debug.NewTranslator(common.WithName(t.name)))
			}
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
		translators.Extensions.Set(agenthealth.NewTranslator(component.DataTypeMetrics, []string{agenthealth.OperationPutMetricData}))
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(component.MustNewType("statuscode"), nil, true))
	case common.AMPKey:
		if conf.IsSet(common.MetricsAggregationDimensionsKey) {
			translators.Processors.Set(rollupprocessor.NewTranslator())
		}
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.MetricsKey))
		translators.Exporters.Set(prometheusremotewrite.NewTranslatorWithName(common.AMPKey))
		translators.Extensions.Set(sigv4auth.NewTranslator())
	case common.CloudWatchLogsKey:
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(t.name, common.LogsKey))
		translators.Exporters.Set(awsemf.NewTranslator())
		translators.Extensions.Set(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents}))
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(component.MustNewType("statuscode"), nil, true))
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
