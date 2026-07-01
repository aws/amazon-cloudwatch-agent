// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

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
	ScrapeTimeout      string
	NodeName           string
	HostIP             string
	AppLogGroup        string
	AppLogStream       string
	NodeLogGroup       string
	NodeLogStream      string
}

// rawMapConfig wraps a raw config map and passes it through serialization unchanged.
// This avoids the struct round-trip that causes:
// - Bug 1: expandconverter misinterpreting $1 injected by struct defaults
// - Bug 2: zero-value ErrorMode in transform processor ContextStatements
// - Bug 3: broken operator.Config marshaling for filelog receiver
type rawMapConfig struct {
	data map[string]interface{}
}

var _ component.Config = (*rawMapConfig)(nil)
var _ confmap.Marshaler = rawMapConfig{}

func (r rawMapConfig) Validate() error { return nil } // validation deferred to component factory at runtime

// Marshal implements confmap.Marshaler so that the internal mapstructure encoder
// returns the raw map directly instead of reflecting over struct fields.
// Uses value receiver because the encoder dereferences pointers before checking
// for the Marshaler interface.
func (r rawMapConfig) Marshal(conf *confmap.Conf) error {
	return conf.Merge(confmap.NewFromStringMap(r.data))
}

// escapeDollarDigit escapes bare $ followed by a digit so the expandconverter
// does not interpret regex backreferences (e.g., k8sattributes $1) as env vars.
func escapeDollarDigit(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			out = append(out, '$', '$')
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
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

// clusterNameRegex restricts cluster_name to safe characters, preventing
// OTTL injection and template metacharacter issues in YAML templates.
var clusterNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func getClusterName(conf *confmap.Conf) (string, error) {
	key := common.ConfigKey(ciConfigKey, "cluster_name")
	name, ok := common.GetString(conf, key)
	if !ok || name == "" {
		return "", fmt.Errorf("cluster_name is required for container_insights")
	}
	if !clusterNameRegex.MatchString(name) {
		return "", fmt.Errorf("cluster_name contains invalid characters: %q (must match %s)", name, clusterNameRegex.String())
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

func hasForwardConnector(exporters []string) bool {
	for _, e := range exporters {
		if e == "forward/opentelemetry" {
			return true
		}
	}
	return false
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

		cfg := &rawMapConfig{data: cfgMap}
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
