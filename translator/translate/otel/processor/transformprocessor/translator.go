// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	_ "embed"
	"fmt"
	"runtime"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

//go:embed transform_jmx_config.yaml
var transformJmxConfig string

//go:embed transform_jmx_drop_config.yaml
var transformJmxDropConfig string

//go:embed transform_efa_config.yaml
var transformEfaConfig string

//go:embed transform_dbi_fix_start_time_postgresql.yaml
var transformDbiFixStartTimeConfig string

//go:embed transform_dbi_fix_start_time_mysql.yaml
var transformDbiFixStartTimeMysqlConfig string

//go:embed transform_identity_ec2.yaml
var transformIdentityEC2Config string

//go:embed transform_identity_azure_vm.yaml
var transformIdentityAzureVMConfig string

//go:embed transform_identity_ecs.yaml
var transformIdentityECSConfig string

//go:embed transform_identity_k8s.yaml
var transformIdentityK8sConfig string

//go:embed transform_logs_routing_host.yaml
var transformLogsRoutingHostConfig string

//go:embed transform_logs_routing_k8s.yaml
var transformLogsRoutingK8sConfig string

type Option func(*translator)

// WithLogResourceStatements sets OTTL statements to execute in the "resource" context for logs.
func WithLogResourceStatements(statements []string) Option {
	return func(t *translator) {
		t.logStatements = statements
	}
}

// WithMetricResourceStatements sets OTTL statements to execute in the "resource" context for metrics.
func WithMetricResourceStatements(statements []string) Option {
	return func(t *translator) {
		t.metricStatements = statements
	}
}

// WithTraceResourceStatements sets OTTL statements to execute in the "resource" context for traces.
func WithTraceResourceStatements(statements []string) Option {
	return func(t *translator) {
		t.traceStatements = statements
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

// WithLogScopeStatements sets OTTL statements to execute in the "scope" context for logs only.
func WithLogScopeStatements(statements []string) Option {
	return func(t *translator) {
		t.logScopeStatements = statements
	}
}

// WithLogContextStatements sets OTTL statements to execute in the "log" context for logs.
func WithLogContextStatements(statements []string) Option {
	return func(t *translator) {
		t.logContextStatements = statements
	}
}

// WithMetricScopeStatements sets OTTL statements to execute in the "scope" context for metrics only.
func WithMetricScopeStatements(statements []string) Option {
	return func(t *translator) {
		t.metricScopeStatements = statements
	}
}

// WithDbiFixStartTime selects the DBI fix-start-time config for the given engine
// ("postgresql" or "mysql"). The component is named by the caller (index-based).
func WithDbiFixStartTime(engine string) Option {
	return func(t *translator) {
		t.dbiFixStartTimeEngine = engine
	}
}

type translator struct {
	name                  string
	factory               processor.Factory
	logStatements         []string
	logContextStatements  []string
	metricStatements      []string
	traceStatements       []string
	scopeStatements       []string
	logScopeStatements    []string
	metricScopeStatements []string
	errorMode             string
	dbiFixStartTimeEngine string
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

func (t *translator) hasDynamicStatements() bool {
	return len(t.logStatements) > 0 ||
		len(t.logContextStatements) > 0 ||
		len(t.metricStatements) > 0 ||
		len(t.traceStatements) > 0 ||
		len(t.scopeStatements) > 0 ||
		len(t.logScopeStatements) > 0 ||
		len(t.metricScopeStatements) > 0
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*transformprocessor.Config)

	if t.hasDynamicStatements() {
		errorMode := t.errorMode
		if errorMode == "" {
			errorMode = "ignore"
		}
		cfgMap := map[string]any{
			"error_mode": errorMode,
		}
		if len(t.metricStatements) > 0 {
			cfgMap["metric_statements"] = []any{buildResourceStatements(t.metricStatements, errorMode)}
		}
		if len(t.logStatements) > 0 {
			cfgMap["log_statements"] = []any{buildResourceStatements(t.logStatements, errorMode)}
		}
		if len(t.traceStatements) > 0 {
			cfgMap["trace_statements"] = []any{buildResourceStatements(t.traceStatements, errorMode)}
		}
		if len(t.scopeStatements) > 0 {
			scopeBlock := buildScopeStatements(t.scopeStatements, errorMode)
			cfgMap["metric_statements"] = appendStatements(cfgMap["metric_statements"], scopeBlock)
			cfgMap["log_statements"] = appendStatements(cfgMap["log_statements"], scopeBlock)
			cfgMap["trace_statements"] = []any{scopeBlock}
		}
		if len(t.metricScopeStatements) > 0 {
			cfgMap["metric_statements"] = appendStatements(cfgMap["metric_statements"], buildScopeStatements(t.metricScopeStatements, errorMode))
		}
		if len(t.logScopeStatements) > 0 {
			cfgMap["log_statements"] = appendStatements(cfgMap["log_statements"], buildScopeStatements(t.logScopeStatements, errorMode))
		}
		if len(t.logContextStatements) > 0 {
			cfgMap["log_statements"] = appendStatements(cfgMap["log_statements"], buildLogStatements(t.logContextStatements, errorMode))
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
	// DBI fix-start-time: content is engine-specific, component name is index-based.
	if t.dbiFixStartTimeEngine != "" {
		switch t.dbiFixStartTimeEngine {
		case common.PostgreSQLKey:
			return common.GetYamlFileToYamlConfig(cfg, transformDbiFixStartTimeConfig)
		case common.MySQLKey:
			return common.GetYamlFileToYamlConfig(cfg, transformDbiFixStartTimeMysqlConfig)
		}
		return nil, fmt.Errorf("unsupported dbi fix-start-time engine: %s", t.dbiFixStartTimeEngine)
	}
	if t.name == common.Identity {
		if context.CurrentContext().KubernetesMode() != "" {
			return common.GetYamlFileToYamlConfig(cfg, transformIdentityK8sConfig)
		}
		if ecsutil.GetECSUtilSingleton().IsECS() {
			return common.GetYamlFileToYamlConfig(cfg, transformIdentityECSConfig)
		}
		switch context.CurrentContext().Mode() {
		case config.ModeAzureVM:
			return common.GetYamlFileToYamlConfig(cfg, transformIdentityAzureVMConfig)
		default:
			return common.GetYamlFileToYamlConfig(cfg, transformIdentityEC2Config)
		}
	}
	if t.name == common.LogsRouting {
		if context.CurrentContext().KubernetesMode() != "" {
			return common.GetYamlFileToYamlConfig(cfg, transformLogsRoutingK8sConfig)
		}
		if runtime.GOOS == "windows" && conf != nil && conf.IsSet(common.WindowsEventsConfigKey) {
			return common.GetYamlFileToYamlConfig(cfg, transformLogsRoutingHostWindowsConfig)
		}
		return common.GetYamlFileToYamlConfig(cfg, transformLogsRoutingHostConfig)
	}

	return cfg, nil
}

func buildResourceStatements(statements []string, errorMode string) map[string]any {
	stmts := make([]any, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	return map[string]any{
		"context":    "resource",
		"error_mode": errorMode,
		"statements": stmts,
	}
}

func buildLogStatements(statements []string, errorMode string) map[string]any {
	stmts := make([]any, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	return map[string]any{
		"context":    "log",
		"error_mode": errorMode,
		"statements": stmts,
	}
}

func buildScopeStatements(statements []string, errorMode string) map[string]any {
	stmts := make([]any, len(statements))
	for i, s := range statements {
		stmts[i] = s
	}
	return map[string]any{
		"context":    "scope",
		"error_mode": errorMode,
		"statements": stmts,
	}
}

func appendStatements(existing any, block map[string]any) []any {
	if existing == nil {
		return []any{block}
	}
	return append(existing.([]any), block)
}
