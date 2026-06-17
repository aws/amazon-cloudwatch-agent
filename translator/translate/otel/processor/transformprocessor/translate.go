// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed transform_jmx_config.yaml
var transformJmxConfig string

//go:embed transform_jmx_drop_config.yaml
var transformJmxDropConfig string

//go:embed transform_efa_config.yaml
var transformEfaConfig string

//go:embed transform_dbi_fix_start_time.yaml
var transformDbiFixStartTimeConfig string

//go:embed transform_identity_host.yaml
var transformIdentityHostConfig string

//go:embed transform_identity_k8s.yaml
var transformIdentityK8sConfig string

//go:embed transform_logs_routing_host.yaml
var transformLogsRoutingHostConfig string

//go:embed transform_logs_routing_k8s.yaml
var transformLogsRoutingK8sConfig string

type Option func(*translator)

// WithLogStatements sets OTTL statements to execute in the "resource" context for logs.
func WithLogStatements(statements []string) Option {
	return func(t *translator) {
		t.logStatements = statements
	}
}

// WithMetricStatements sets OTTL statements to execute in the "resource" context for metrics.
func WithMetricStatements(statements []string) Option {
	return func(t *translator) {
		t.metricStatements = statements
	}
}

// WithErrorMode sets the error mode for dynamic statements. Defaults to "propagate".
func WithErrorMode(mode string) Option {
	return func(t *translator) {
		t.errorMode = mode
	}
}

// WithScopeStatements sets OTTL statements to execute in the "scope" context for all signal types.
func WithScopeStatements(statements []string) Option {
	return func(t *translator) {
		t.scopeStatements = statements
	}
}

type translator struct {
	name             string
	factory          processor.Factory
	logStatements    []string
	metricStatements []string
	scopeStatements  []string
	errorMode        string
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithName(name string, opts ...Option) common.ComponentTranslator {
	t := &translator{name: name, factory: transformprocessor.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*transformprocessor.Config)

	// Dynamic statements (generic path for both metrics and logs)
	if len(t.logStatements) > 0 || len(t.metricStatements) > 0 || len(t.scopeStatements) > 0 {
		errorMode := t.errorMode
		if errorMode == "" {
			errorMode = "propagate"
		}
		cfgMap := map[string]interface{}{
			"error_mode": errorMode,
		}
		if len(t.metricStatements) > 0 {
			cfgMap["metric_statements"] = []interface{}{buildStatements(t.metricStatements)}
		}
		if len(t.logStatements) > 0 {
			cfgMap["log_statements"] = []interface{}{buildStatements(t.logStatements)}
		}
		if len(t.scopeStatements) > 0 {
			scopeBlock := buildScopeStatements(t.scopeStatements)
			cfgMap["metric_statements"] = appendStatements(cfgMap["metric_statements"], scopeBlock)
			cfgMap["log_statements"] = appendStatements(cfgMap["log_statements"], scopeBlock)
			cfgMap["trace_statements"] = []interface{}{scopeBlock}
		}
		if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
			return nil, fmt.Errorf("failed to configure transform processor: %w", err)
		}
		return cfg, nil
	}

	// Static YAML configs
	if t.name == common.PipelineNameContainerInsightsJmx {
		return common.GetYamlFileToYamlConfig(cfg, transformJmxConfig)
	}
	if strings.HasPrefix(t.name, common.PipelineNameJmx) {
		return common.GetYamlFileToYamlConfig(cfg, transformJmxDropConfig)
	}
	if strings.HasPrefix(t.name, common.PipelineNameHostDeltaMetrics) {
		return common.GetYamlFileToYamlConfig(cfg, transformEfaConfig)
	}
	if t.name == common.DbiTransformFixStartTime {
		return common.GetYamlFileToYamlConfig(cfg, transformDbiFixStartTimeConfig)
	}
	if t.name == common.Identity {
		if context.CurrentContext().KubernetesMode() != "" {
			return common.GetYamlFileToYamlConfig(cfg, transformIdentityK8sConfig)
		}
		return common.GetYamlFileToYamlConfig(cfg, transformIdentityHostConfig)
	}
	if t.name == common.LogsRouting {
		if context.CurrentContext().KubernetesMode() != "" {
			return common.GetYamlFileToYamlConfig(cfg, transformLogsRoutingK8sConfig)
		}
		return common.GetYamlFileToYamlConfig(cfg, transformLogsRoutingHostConfig)
	}

	return cfg, nil
}

func buildStatements(statements []string) map[string]interface{} {
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	return map[string]interface{}{
		"context":    "resource",
		"error_mode": "propagate",
		"statements": stmts,
	}
}

func buildScopeStatements(statements []string) map[string]interface{} {
	stmts := make([]interface{}, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	return map[string]interface{}{
		"context":    "scope",
		"error_mode": "ignore",
		"statements": stmts,
	}
}

func appendStatements(existing interface{}, block map[string]interface{}) []interface{} {
	if existing == nil {
		return []interface{}{block}
	}
	return append(existing.([]interface{}), block)
}
