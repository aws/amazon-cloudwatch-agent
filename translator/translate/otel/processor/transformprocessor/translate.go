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

type translator struct {
	name             string
	factory          processor.Factory
	logStatements    []string
	metricStatements []string
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

	// Dynamic log statements
	if len(t.logStatements) > 0 {
		stmts := make([]interface{}, len(t.logStatements))
		for i, s := range t.logStatements {
			stmts[i] = s
		}

		// Application Signals: sets log group/stream with propagate error handling
		if strings.HasPrefix(t.name, "application_signals_logs") {
			cfgMap := map[string]interface{}{
				"log_statements": []interface{}{
					map[string]interface{}{
						"context":    "resource",
						"error_mode": "propagate",
						"statements": stmts,
					},
				},
			}
			if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
				return nil, fmt.Errorf("failed to configure transform processor: %w", err)
			}
			return cfg, nil
		}

		// DBI: sets log group/stream with ignore error handling
		if strings.HasPrefix(t.name, common.DbiTransformLogs) {
			cfgMap := map[string]interface{}{
				"error_mode": "propagate",
				"log_statements": []interface{}{
					map[string]interface{}{
						"context":    "resource",
						"error_mode": "ignore",
						"statements": stmts,
					},
				},
			}
			if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
				return nil, fmt.Errorf("failed to configure transform processor: %w", err)
			}
			return cfg, nil
		}
	}

	// Dynamic metric statements
	if len(t.metricStatements) > 0 {
		stmts := make([]interface{}, len(t.metricStatements))
		for i, s := range t.metricStatements {
			stmts[i] = s
		}

		// DBI: sets db.system.name and db.instance.name on both metrics and logs
		if strings.HasPrefix(t.name, common.DbiTransformResource) {
			context := map[string]interface{}{
				"context":    "resource",
				"error_mode": "ignore",
				"statements": stmts,
			}
			cfgMap := map[string]interface{}{
				"error_mode":        "propagate",
				"metric_statements": []interface{}{context},
				"log_statements":    []interface{}{context},
			}
			if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
				return nil, fmt.Errorf("failed to configure transform processor: %w", err)
			}
			return cfg, nil
		}
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

	return cfg, nil
}
