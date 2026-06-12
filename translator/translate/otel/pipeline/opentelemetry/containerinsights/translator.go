// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
)

//go:embed kubeletstats.yaml
var kubeletstatsYAML string

//go:embed cadvisor.yaml
var cadvisorYAML string

//go:embed node_exporter.yaml
var nodeExporterYAML string

//go:embed dcgm.yaml
var dcgmYAML string

//go:embed neuron.yaml
var neuronYAML string

//go:embed efa.yaml
var efaYAML string

//go:embed ebs_csi.yaml
var ebsCsiYAML string

//go:embed lis_csi.yaml
var lisCsiYAML string

// CI logs pipelines are self-contained with dedicated exporters (compression: none)
// to match the helm chart behavior for FluentBit migration parity. They cannot share
// the base logs/opentelemetry exporter which uses gzip compression. See:
// https://github.com/aws-observability/helm-charts/blob/main/charts/amazon-cloudwatch-observability/templates/linux/_otel-container-insights-config.tpl

//go:embed filelog_app.yaml
var filelogAppYAML string

//go:embed filelog_node.yaml
var filelogNodeYAML string

//go:embed apiserver.yaml
var apiserverYAML string

//go:embed kube_state_metrics.yaml
var kubeStateMetricsYAML string

// NewTranslators returns all container insights pipeline translators.
// The pipelines generated depend on the "mode" config field:
//   - "node": daemonset pipelines (per-node metrics + logs)
//   - "cluster": deployment pipelines (cluster-wide metrics)
//   - omitted: all pipelines
func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	mode := getMode(conf)

	// Daemonset metrics pipelines
	if mode == "" || mode == "node" {
		translators.Set(newYAMLPipeline("kubeletstats", pipeline.SignalMetrics, kubeletstatsYAML))
		translators.Set(newYAMLPipeline("cadvisor", pipeline.SignalMetrics, cadvisorYAML))
		translators.Set(newYAMLPipeline("node_exporter", pipeline.SignalMetrics, nodeExporterYAML))
		translators.Set(newYAMLPipeline("dcgm", pipeline.SignalMetrics, dcgmYAML))
		translators.Set(newYAMLPipeline("neuron", pipeline.SignalMetrics, neuronYAML))
		translators.Set(newYAMLPipeline("efa", pipeline.SignalMetrics, efaYAML))
		translators.Set(newYAMLPipeline("ebs_csi_node", pipeline.SignalMetrics, ebsCsiYAML))
		translators.Set(newYAMLPipeline("lis_csi_node", pipeline.SignalMetrics, lisCsiYAML))

		// Daemonset logs pipelines (gated by logs.enabled)
		if logsEnabled(conf) {
			translators.Set(newYAMLPipeline("app", pipeline.SignalLogs, filelogAppYAML))
			translators.Set(newYAMLPipeline("node", pipeline.SignalLogs, filelogNodeYAML))
		}
	}

	// Deployment metrics pipelines
	if mode == "" || mode == "cluster" {
		translators.Set(newYAMLPipeline("apiserver", pipeline.SignalMetrics, apiserverYAML))
		translators.Set(newYAMLPipeline("kube_state_metrics", pipeline.SignalMetrics, kubeStateMetricsYAML))
	}

	return translators
}

// yamlPipelineTranslator implements PipelineTranslator using an embedded YAML template.
type yamlPipelineTranslator struct {
	name     string
	signal   pipeline.Signal
	yamlTmpl string
}

var _ common.PipelineTranslator = (*yamlPipelineTranslator)(nil)

func newYAMLPipeline(name string, signal pipeline.Signal, yamlTmpl string) common.PipelineTranslator {
	return &yamlPipelineTranslator{name: name, signal: signal, yamlTmpl: yamlTmpl}
}

func (t *yamlPipelineTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(t.signal, ciPrefix+"_"+t.name)
}

func (t *yamlPipelineTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(ciConfigKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: ciConfigKey}
	}

	clusterName, err := getClusterName(conf)
	if err != nil {
		return nil, err
	}

	data := templateData{
		ClusterName:        clusterName,
		Region:             agent.Global_Config.Region,
		CollectionInterval: getCollectionInterval(conf).String(),
		NodeName:           "${env:K8S_NODE_NAME}",
		HostIP:             "${env:HOST_IP}",
		AppLogGroup:        fmt.Sprintf("/aws/otel/containerinsights/%s/application", clusterName),
		AppLogStream:       "${env:K8S_NODE_NAME}-application",
		NodeLogGroup:       fmt.Sprintf("/aws/otel/containerinsights/%s/host", clusterName),
		NodeLogStream:      "${env:K8S_NODE_NAME}-host",
	}

	// Execute template
	tmpl, err := template.New(t.name).Parse(t.yamlTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template for %s: %w", t.name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template for %s: %w", t.name, err)
	}

	// Parse YAML
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse YAML for %s: %w", t.name, err)
	}

	// Extract pipeline ordering from service section
	pipelineOrder, err := extractPipelineOrder(parsed, t.ID().String())
	if err != nil {
		return nil, err
	}

	// Build component translators for each section
	receivers, err := buildComponentTranslators(parsed, "receivers", pipelineOrder.receivers)
	if err != nil {
		return nil, fmt.Errorf("pipeline %s receivers: %w", t.name, err)
	}
	processors, err := buildComponentTranslators(parsed, "processors", pipelineOrder.processors)
	if err != nil {
		return nil, fmt.Errorf("pipeline %s processors: %w", t.name, err)
	}
	var exporters common.ComponentTranslatorMap
	var connectors common.ComponentTranslatorMap
	if hasForwardConnector(pipelineOrder.exporters) {
		fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)
		exporters = common.NewTranslatorMap[component.Config, component.ID](fwdConnector)
		connectors = common.NewTranslatorMap[component.Config, component.ID](fwdConnector)
	} else {
		var err error
		exporters, err = buildComponentTranslators(parsed, "exporters", pipelineOrder.exporters)
		if err != nil {
			return nil, fmt.Errorf("pipeline %s exporters: %w", t.name, err)
		}
	}
	extensions, err := buildComponentTranslators(parsed, "extensions", nil)
	if err != nil {
		return nil, fmt.Errorf("pipeline %s extensions: %w", t.name, err)
	}

	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
		Extensions: extensions,
		Connectors: connectors,
	}, nil
}
