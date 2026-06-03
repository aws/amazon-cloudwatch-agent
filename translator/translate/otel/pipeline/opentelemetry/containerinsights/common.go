// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscloudwatchlogsprovisionerextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributestocontextprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsattributelimitprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsdevicepodcorrelationprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/amazon-cloudwatch-agent/extension/nodemetadatacache"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsneuron"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/nodemetadataenricher"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	ciPrefix                  = "cw_k8s_ci_v0"
	defaultCollectionInterval = 30 * time.Second
)

var ciConfigKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtelContainerInsightsKey)

// templateData holds the dynamic values for YAML template execution.
type templateData struct {
	ClusterName        string
	Region             string
	CollectionInterval string
	NodeName           string
	HostIP             string
	AppLogGroup        string
	AppLogStream       string
	NodeLogGroup       string
	NodeLogStream      string
}

// factoryEntry holds a component factory for creating default configs.
type factoryEntry struct {
	createDefaultConfig func() component.Config
}

// factoryRegistry maps component type names to their factories.
var factoryRegistry = map[string]factoryEntry{
	// Receivers
	"kubeletstats":   {createDefaultConfig: kubeletstatsreceiver.NewFactory().CreateDefaultConfig},
	"prometheus":     {createDefaultConfig: prometheusreceiver.NewFactory().CreateDefaultConfig},
	"filelog":        {createDefaultConfig: filelogreceiver.NewFactory().CreateDefaultConfig},
	"awsefareceiver": {createDefaultConfig: awsefareceiver.NewFactory().CreateDefaultConfig},
	// Processors
	"transform":               {createDefaultConfig: transformprocessor.NewFactory().CreateDefaultConfig},
	"filter":                  {createDefaultConfig: filterprocessor.NewFactory().CreateDefaultConfig},
	"batch":                   {createDefaultConfig: batchprocessor.NewFactory().CreateDefaultConfig},
	"k8sattributes":           {createDefaultConfig: k8sattributesprocessor.NewFactory().CreateDefaultConfig},
	"resourcedetection":       {createDefaultConfig: resourcedetectionprocessor.NewFactory().CreateDefaultConfig},
	"groupbyattrs":            {createDefaultConfig: groupbyattrsprocessor.NewFactory().CreateDefaultConfig},
	"metricstarttime":         {createDefaultConfig: metricstarttimeprocessor.NewFactory().CreateDefaultConfig},
	"awsattributelimit":       {createDefaultConfig: awsattributelimitprocessor.NewFactory().CreateDefaultConfig},
	"awsdevicepodcorrelation": {createDefaultConfig: awsdevicepodcorrelationprocessor.NewFactory().CreateDefaultConfig},
	"awsneuron":               {createDefaultConfig: awsneuron.NewFactory().CreateDefaultConfig},
	"nodemetadataenricher":    {createDefaultConfig: nodemetadataenricher.NewFactory().CreateDefaultConfig},
	"attributestocontext":     {createDefaultConfig: attributestocontextprocessor.NewFactory().CreateDefaultConfig},
	// Exporters
	"otlphttp": {createDefaultConfig: otlphttpexporter.NewFactory().CreateDefaultConfig},
	// Extensions
	"sigv4auth":                    {createDefaultConfig: sigv4authextension.NewFactory().CreateDefaultConfig},
	"awscloudwatchlogsprovisioner": {createDefaultConfig: awscloudwatchlogsprovisionerextension.NewFactory().CreateDefaultConfig},
	"nodemetadatacache":            {createDefaultConfig: nodemetadatacache.NewFactory().CreateDefaultConfig},
	"headers_setter":               {createDefaultConfig: headerssetterextension.NewFactory().CreateDefaultConfig},
}

// yamlComponentTranslator wraps a pre-built component config.
type yamlComponentTranslator struct {
	id  component.ID
	cfg component.Config
}

var _ common.ComponentTranslator = (*yamlComponentTranslator)(nil)

func (t *yamlComponentTranslator) ID() component.ID { return t.id }
func (t *yamlComponentTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	return t.cfg, nil
}

// createComponentConfig looks up the factory for the component type, creates a default config,
// and unmarshals the YAML section into it.
func createComponentConfig(id component.ID, cfgMap map[string]interface{}) (component.Config, error) {
	entry, ok := factoryRegistry[id.Type().String()]
	if !ok {
		return nil, fmt.Errorf("unknown component type: %s", id.Type())
	}
	cfg := entry.createDefaultConfig()
	if len(cfgMap) > 0 {
		if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config for %s: %w", id, err)
		}
	}
	return cfg, nil
}

func getClusterName(conf *confmap.Conf) (string, error) {
	key := common.ConfigKey(ciConfigKey, "cluster_name")
	name, ok := common.GetString(conf, key)
	if !ok || name == "" {
		return "", fmt.Errorf("cluster_name is required for container_insights")
	}
	return name, nil
}

func getCollectionInterval(conf *confmap.Conf) time.Duration {
	key := common.ConfigKey(ciConfigKey, "collection_interval")
	if v, ok := common.GetNumber(conf, key); ok && v > 0 {
		return time.Duration(v) * time.Second
	}
	return defaultCollectionInterval
}

// logsEnabled returns true if container_insights.logs.enabled is set to true.
func logsEnabled(conf *confmap.Conf) bool {
	if conf == nil {
		return false
	}
	key := common.ConfigKey(ciConfigKey, "logs", "enabled")
	return common.GetOrDefaultBool(conf, key, false)
}

// getMode returns the container_insights.mode value ("node", "cluster", or "" for all).
func getMode(conf *confmap.Conf) string {
	if conf == nil {
		return ""
	}
	key := common.ConfigKey(ciConfigKey, "mode")
	if v, ok := common.GetString(conf, key); ok {
		return v
	}
	return ""
}
