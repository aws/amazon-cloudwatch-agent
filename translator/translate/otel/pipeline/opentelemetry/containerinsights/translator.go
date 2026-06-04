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
	exporters, err := buildComponentTranslators(parsed, "exporters", pipelineOrder.exporters)
	if err != nil {
		return nil, fmt.Errorf("pipeline %s exporters: %w", t.name, err)
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
	}, nil
}

type pipelineSpec struct {
	receivers  []string
	processors []string
	exporters  []string
}

func extractPipelineOrder(parsed map[string]interface{}, pipelineID string) (*pipelineSpec, error) {
	svc, ok := parsed["service"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing service section")
	}
	pipelines, ok := svc["pipelines"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing service.pipelines section")
	}

	// Find our pipeline
	var pipelineCfg map[string]interface{}
	for name, v := range pipelines {
		if name == pipelineID {
			pipelineCfg, _ = v.(map[string]interface{})
			break
		}
	}
	if pipelineCfg == nil {
		return nil, fmt.Errorf("pipeline %s not found in service.pipelines", pipelineID)
	}

	return &pipelineSpec{
		receivers:  toStringSlice(pipelineCfg["receivers"]),
		processors: toStringSlice(pipelineCfg["processors"]),
		exporters:  toStringSlice(pipelineCfg["exporters"]),
	}, nil
}

func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// buildComponentTranslators creates translators for each component in a section,
// ordered by the pipeline spec ordering if provided.
func buildComponentTranslators(parsed map[string]interface{}, section string, order []string) (common.ComponentTranslatorMap, error) {
	sectionMap, _ := parsed[section].(map[string]interface{})
	if sectionMap == nil {
		return common.NewTranslatorMap[component.Config, component.ID](), nil
	}

	// Determine order: use pipeline spec order if provided, otherwise use map keys
	var keys []string
	if len(order) > 0 {
		keys = order
	} else {
		keys = make([]string, 0, len(sectionMap))
		for k := range sectionMap {
			keys = append(keys, k)
		}
	}

	translators := make([]common.ComponentTranslator, 0, len(keys))
	for _, fullName := range keys {
		cfgMap, _ := sectionMap[fullName].(map[string]interface{})
		if cfgMap == nil {
			cfgMap = map[string]interface{}{}
		}

		id, err := parseComponentID(fullName)
		if err != nil {
			return common.NewTranslatorMap[component.Config, component.ID](), err
		}

		cfg, err := createComponentConfig(id, cfgMap)
		if err != nil {
			return common.NewTranslatorMap[component.Config, component.ID](), err
		}

		translators = append(translators, &yamlComponentTranslator{id: id, cfg: cfg})
	}

	return common.NewTranslatorMap[component.Config, component.ID](translators...), nil
}

// parseComponentID parses "type/name" or "type" into a component.ID.
func parseComponentID(fullName string) (component.ID, error) {
	var id component.ID
	if err := id.UnmarshalText([]byte(fullName)); err != nil {
		return component.ID{}, fmt.Errorf("failed to parse component ID %q: %w", fullName, err)
	}
	return id, nil
}
