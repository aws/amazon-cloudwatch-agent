// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package deltatosparseprocessor

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/deltatosparseprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsight"
)

const (
	// Match types are in internal package from contrib
	// Strict is the FilterType for filtering by exact string matches.
	strict = "strict"
	regexp = "regexp"
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, deltatosparseprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*deltatosparseprocessor.Config)
	if awscontainerinsight.EnhancedContainerInsightsEnabled(conf) && awscontainerinsight.AcceleratedComputeMetricsEnabled(conf) {
		includeMetrics := []string{
			"node_neuron_execution_errors_generic",
			"node_neuron_execution_errors_numerical",
			"node_neuron_execution_errors_transient",
			"node_neuron_execution_errors_model",
			"node_neuron_execution_errors_runtime",
			"node_neuron_execution_errors_hardware",
			"node_neuron_execution_status_completed",
			"node_neuron_execution_status_timed_out",
			"node_neuron_execution_status_completed_with_err",
			"node_neuron_execution_status_completed_with_num_err",
			"node_neuron_execution_status_incorrect_input",
			"node_neuron_execution_status_failed_to_queue",
		}
		cfg.Include = includeMetrics
	}
	return cfg, nil
}
