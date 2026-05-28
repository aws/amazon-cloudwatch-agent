// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

func TestNewTranslatorsNilConf(t *testing.T) {
	translators, _ := NewTranslators(nil)
	assert.Equal(t, 0, translators.Len())
}

func TestNewTranslatorsNoSyslogKey(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)
	assert.Equal(t, 0, translators.Len())
}

func TestNewTranslatorsSingleListenerNoRules(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": map[string]any{
				"listen_address": "tcp://0.0.0.0:514",
				"log_group_name": "/default",
			},
		}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)
	// No routing rules → single direct pipeline
	assert.Equal(t, 1, translators.Len())
	keys := collections.MapSlice(translators.Keys(), pipeline.ID.String)
	assert.Equal(t, []string{"logs/syslog_0_default"}, keys)
}

func TestNewTranslatorsWithRoutingRules(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": map[string]any{
				"listen_address": "tcp://0.0.0.0:514",
				"log_group_name": "/default",
				"routing": []any{
					map[string]any{
						"match":          map[string]any{"hostname": "web-*"},
						"log_group_name": "/web",
					},
					map[string]any{
						"match":          map[string]any{"facility": float64(4)},
						"log_group_name": "/auth",
					},
				},
			},
		}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)
	// input + 2 rules + 1 default = 4 pipelines
	assert.Equal(t, 4, translators.Len())
	keys := collections.MapSlice(translators.Keys(), pipeline.ID.String)
	assert.Equal(t, []string{
		"logs/syslog_0_in",
		"logs/syslog_0_rule_0",
		"logs/syslog_0_rule_1",
		"logs/syslog_0_default",
	}, keys)
}

func TestInputPipelineTranslate(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": map[string]any{
				"listen_address": "tcp://0.0.0.0:514",
				"log_group_name": "/default",
				"routing": []any{
					map[string]any{
						"match":          map[string]any{"hostname": "web-*"},
						"log_group_name": "/web",
					},
				},
			},
		}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)

	inputPipeline, ok := translators.Get(pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_in"))
	require.True(t, ok)
	got, err := inputPipeline.Translate(conf)
	require.NoError(t, err)

	// Input pipeline has receiver, routing connector as exporter
	assert.Equal(t, []string{"syslog/tcp_0_0_0_0_514"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Equal(t, []string{"routing/syslog_0"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	// Connector should be registered
	assert.Equal(t, []string{"routing/syslog_0"}, collections.MapSlice(got.Connectors.Keys(), component.ID.String))
}

func TestOutputPipelineTranslate(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": map[string]any{
				"listen_address": "tcp://0.0.0.0:514",
				"log_group_name": "/default",
				"routing": []any{
					map[string]any{
						"match":          map[string]any{"hostname": "web-*"},
						"log_group_name": "/web",
					},
				},
			},
		}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)

	// Check rule pipeline
	rulePipeline, ok := translators.Get(pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_rule_0"))
	require.True(t, ok)
	got, err := rulePipeline.Translate(conf)
	require.NoError(t, err)

	// Output pipeline has routing connector as receiver
	assert.Equal(t, []string{"routing/syslog_0"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	// Has batch processor
	procIDs := collections.MapSlice(got.Processors.Keys(), component.ID.String)
	assert.Contains(t, procIDs, "batch/syslog_0_rule_0")
	// Has CWL exporter
	assert.Equal(t, []string{"awscloudwatchlogs/syslog_0_rule_0"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
}

func TestDeriveReceiverName(t *testing.T) {
	assert.Equal(t, "tcp_0_0_0_0_514", deriveReceiverName("tcp://0.0.0.0:514"))
	assert.Equal(t, "udp_127_0_0_1_6515", deriveReceiverName("udp://127.0.0.1:6515"))
}

func TestBuildOTTLCondition(t *testing.T) {
	tests := []struct {
		name  string
		match map[string]any
		want  string
	}{
		{
			name:  "hostname glob",
			match: map[string]any{"hostname": "web-*"},
			want:  `IsMatch(attributes["hostname"], "web-*")`,
		},
		{
			name:  "hostname exact",
			match: map[string]any{"hostname": "myhost"},
			want:  `attributes["hostname"] == "myhost"`,
		},
		{
			name:  "facility",
			match: map[string]any{"facility": float64(4)},
			want:  `attributes["facility"] == 4`,
		},
		{
			name:  "multiple conditions",
			match: map[string]any{"hostname": "web-*", "app_name": "nginx"},
			want:  `IsMatch(attributes["hostname"], "web-*") and attributes["app_name"] == "nginx"`,
		},
		{
			name:  "empty",
			match: map[string]any{},
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildOTTLCondition(tc.match)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsGlobPattern(t *testing.T) {
	assert.True(t, isGlobPattern("web-*"))
	assert.True(t, isGlobPattern("host[0-9]"))
	assert.True(t, isGlobPattern("app?"))
	assert.False(t, isGlobPattern("exact-match"))
}

func TestToFilters(t *testing.T) {
	testCases := []struct {
		name  string
		input map[string]any
		want  int
	}{
		{
			name:  "NoFiltersKey",
			input: map[string]any{},
			want:  0,
		},
		{
			name:  "FiltersNotSlice",
			input: map[string]any{"filters": "invalid"},
			want:  0,
		},
		{
			name: "ValidFilters",
			input: map[string]any{
				"filters": []any{
					map[string]any{"type": "exclude", "expression": "healthcheck"},
					map[string]any{"type": "include", "expression": "error|crit"},
				},
			},
			want: 2,
		},
		{
			name: "SkipsInvalidEntry",
			input: map[string]any{
				"filters": []any{
					"not a map",
					map[string]any{"type": "include", "expression": "error"},
				},
			},
			want: 1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := toFilters(tc.input)
			assert.Len(t, got, tc.want)
		})
	}
}

func TestNewTranslatorsWithTLS(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": map[string]any{
				"listen_address": "tcp://0.0.0.0:6514",
				"log_group_name": "/tls/default",
				"tls": map[string]any{
					"cert_file":   "/etc/ssl/cert.pem",
					"key_file":    "/etc/ssl/key.pem",
					"ca_file":     "/etc/ssl/ca.pem",
					"min_version": "1.3",
				},
			},
		}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)
	require.Equal(t, 1, translators.Len()) // single direct pipeline (no rules)

	// Get the default pipeline and verify receiver
	defaultPipeline, ok := translators.Get(pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_default"))
	require.True(t, ok)

	got, err := defaultPipeline.Translate(conf)
	require.NoError(t, err)

	receiverIDs := collections.MapSlice(got.Receivers.Keys(), component.ID.String)
	assert.Equal(t, []string{"syslog/tcp_0_0_0_0_6514"}, receiverIDs)

	// Translate the receiver and verify TLS fields propagate
	receiverTrans, ok := got.Receivers.Get(component.NewIDWithName(component.MustNewType("syslog"), "tcp_0_0_0_0_6514"))
	require.True(t, ok)

	receiverCfg, err := receiverTrans.Translate(confmap.New())
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(receiverCfg))
	assert.Equal(t, "/etc/ssl/cert.pem", out.Get("tcp::tls::cert_file"))
	assert.Equal(t, "/etc/ssl/key.pem", out.Get("tcp::tls::key_file"))
	assert.Equal(t, "/etc/ssl/ca.pem", out.Get("tcp::tls::ca_file"))
	assert.Equal(t, "1.3", out.Get("tcp::tls::min_version"))
}

func TestFilterProcessorTranslator_ID(t *testing.T) {
	tr := newFilterProcessorTranslator("syslog_in", []filter{{Type: "exclude", Expression: "health"}})
	assert.Equal(t, "filter/syslog_in", tr.ID().String())
}

func TestFilterProcessorTranslator_Translate(t *testing.T) {
	tests := []struct {
		name    string
		filters []filter
		want    []string
	}{
		{
			name:    "exclude filter",
			filters: []filter{{Type: "exclude", Expression: "healthcheck"}},
			want:    []string{`IsMatch(body, "healthcheck")`},
		},
		{
			name:    "include filter",
			filters: []filter{{Type: "include", Expression: "error|warn"}},
			want:    []string{`not IsMatch(body, "error|warn")`},
		},
		{
			name: "mixed filters",
			filters: []filter{
				{Type: "exclude", Expression: "debug"},
				{Type: "include", Expression: "crit"},
			},
			want: []string{`IsMatch(body, "debug")`, `not IsMatch(body, "crit")`},
		},
		{
			name:    "empty filters",
			filters: []filter{},
			want:    nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newFilterProcessorTranslator("test", tc.filters)
			cfg, err := tr.Translate(confmap.New())
			require.NoError(t, err)

			out := confmap.New()
			require.NoError(t, out.Marshal(cfg))
			conditions, _ := out.Get("logs::log_record").([]any)
			if tc.want == nil {
				assert.Nil(t, conditions)
			} else {
				require.Len(t, conditions, len(tc.want))
				for i, want := range tc.want {
					assert.Equal(t, want, conditions[i])
				}
			}
		})
	}
}

func TestRoutingConnectorTranslator_ID(t *testing.T) {
	tr := newRoutingConnectorTranslator("syslog_0", nil, nil)
	assert.Equal(t, "routing/syslog_0", tr.ID().String())
}

func TestRoutingConnectorTranslator_Translate(t *testing.T) {
	defaultPipelines := []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_default")}
	table := []routingTableEntry{
		{condition: `IsMatch(attributes["hostname"], "web-*")`, pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_rule_0")}},
		{condition: `attributes["facility"] == "4"`, pipelines: []pipeline.ID{pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_rule_1")}},
	}
	tr := newRoutingConnectorTranslator("syslog_0", defaultPipelines, table)
	cfg, err := tr.Translate(confmap.New())
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(cfg))
	assert.Equal(t, []any{"logs/syslog_0_default"}, out.Get("default_pipelines"))
	assert.Contains(t, fmt.Sprint(out.Get("error_mode")), "ignore")

	tableOut, ok := out.Get("table").([]any)
	require.True(t, ok)
	require.Len(t, tableOut, 2)

	entry0 := tableOut[0].(map[string]any)
	assert.Equal(t, "log", entry0["context"])
	assert.Equal(t, `IsMatch(attributes["hostname"], "web-*")`, entry0["condition"])
}

func TestSigV4AuthTranslator_ID(t *testing.T) {
	tr := newSigV4AuthTranslator("us-east-1", "")
	assert.Equal(t, "sigv4auth/syslog", tr.ID().String())
}

func TestSigV4AuthTranslator_Translate(t *testing.T) {
	tr := newSigV4AuthTranslator("us-west-2", "arn:aws:iam::123456789:role/MyRole")
	cfg, err := tr.Translate(confmap.New())
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(cfg))
	assert.Equal(t, "us-west-2", out.Get("region"))
	assert.Equal(t, "logs", out.Get("service"))
	assert.Equal(t, "arn:aws:iam::123456789:role/MyRole", out.Get("assume_role::arn"))
	assert.Equal(t, "us-west-2", out.Get("assume_role::sts_region"))
}

func TestSigV4AuthTranslator_NoRoleARN(t *testing.T) {
	tr := newSigV4AuthTranslator("eu-west-1", "")
	cfg, err := tr.Translate(confmap.New())
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(cfg))
	assert.Equal(t, "eu-west-1", out.Get("region"))
	assert.Equal(t, "logs", out.Get("service"))
	assert.Empty(t, out.Get("assume_role::arn"))
}

func TestBuildFilterConditions(t *testing.T) {
	tests := []struct {
		name    string
		filters []filter
		want    []string
	}{
		{
			name:    "empty expression skipped",
			filters: []filter{{Type: "exclude", Expression: ""}},
			want:    nil,
		},
		{
			name:    "escapes quotes",
			filters: []filter{{Type: "exclude", Expression: `say "hello"`}},
			want:    []string{`IsMatch(body, "say \"hello\"")`},
		},
		{
			name:    "escapes backslash",
			filters: []filter{{Type: "include", Expression: `path\\file`}},
			want:    []string{`not IsMatch(body, "path\\\\file")`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildFilterConditions(tc.filters)
			if tc.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestNewTranslatorsMultiSection(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": []any{
				map[string]any{
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/infra/default",
					"routing": []any{
						map[string]any{
							"match":          map[string]any{"facility": float64(4)},
							"log_group_name": "/infra/auth",
						},
					},
				},
				map[string]any{
					"listeners": []any{
						map[string]any{"listen_address": "tcp://0.0.0.0:1514"},
						map[string]any{"listen_address": "udp://0.0.0.0:1514"},
					},
					"log_group_name": "/apps/default",
					"routing": []any{
						map[string]any{
							"match":          map[string]any{"hostname": "web-*"},
							"log_group_name": "/apps/web",
						},
						map[string]any{
							"match":          map[string]any{"app_name": "api-*"},
							"log_group_name": "/apps/api",
						},
					},
				},
			},
		}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)

	// Section 0: 1 input + 1 rule + 1 default = 3
	// Section 1: 1 input + 2 rules + 1 default = 4
	// Total: 7
	assert.Equal(t, 7, translators.Len())

	keys := collections.MapSlice(translators.Keys(), pipeline.ID.String)
	// Section 0 pipelines
	assert.Contains(t, keys, "logs/syslog_0_in")
	assert.Contains(t, keys, "logs/syslog_0_rule_0")
	assert.Contains(t, keys, "logs/syslog_0_default")
	// Section 1 pipelines
	assert.Contains(t, keys, "logs/syslog_1_in")
	assert.Contains(t, keys, "logs/syslog_1_rule_0")
	assert.Contains(t, keys, "logs/syslog_1_rule_1")
	assert.Contains(t, keys, "logs/syslog_1_default")

	// Verify section 1 input pipeline has both receivers
	inputPipeline, ok := translators.Get(pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_1_in"))
	require.True(t, ok)
	got, err := inputPipeline.Translate(conf)
	require.NoError(t, err)
	receiverIDs := collections.MapSlice(got.Receivers.Keys(), component.ID.String)
	assert.Contains(t, receiverIDs, "syslog/tcp_0_0_0_0_1514")
	assert.Contains(t, receiverIDs, "syslog/udp_0_0_0_0_1514")

	// Verify section 1 uses its own routing connector
	assert.Equal(t, []string{"routing/syslog_1"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
}

func TestNewTranslatorsArrayFormSingleSection(t *testing.T) {
	// Array with one element should work the same as single object
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": []any{
				map[string]any{
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/default",
				},
			},
		}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)
	assert.Equal(t, 1, translators.Len())
	keys := collections.MapSlice(translators.Keys(), pipeline.ID.String)
	assert.Equal(t, []string{"logs/syslog_0_default"}, keys)
}

func TestNewTranslatorsDuplicateListenerAcrossSections(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": []any{
				map[string]any{
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/section0",
				},
				map[string]any{
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/section1",
				},
			},
		}},
	})
	_, err := NewTranslators(conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tcp://0.0.0.0:514")
	assert.Contains(t, err.Error(), "section 0")
	assert.Contains(t, err.Error(), "section 1")
}

func TestNewTranslatorsUniqueListenersAcrossSections(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": []any{
				map[string]any{
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/section0",
				},
				map[string]any{
					"listen_address": "tcp://0.0.0.0:1514",
					"log_group_name": "/section1",
				},
			},
		}},
	})
	translators, err := NewTranslators(conf)
	require.NoError(t, err)
	assert.Equal(t, 2, translators.Len()) // one simple pipeline per section
}

func TestProvisionerTranslator_ID(t *testing.T) {
	tr := newProvisionerTranslator("syslog_0_default", "us-east-1", "/syslog/default", "default", 7)
	assert.Equal(t, "awscloudwatchlogsprovisioner/syslog_0_default", tr.ID().String())
}

func TestProvisionerTranslator_Translate(t *testing.T) {
	tr := newProvisionerTranslator("syslog_0_rule_0", "us-west-2", "/syslog/web", "web-stream", 30)
	cfg, err := tr.Translate(confmap.New())
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(cfg))
	assert.Equal(t, "us-west-2", out.Get("region"))
	assert.Equal(t, "/syslog/web", out.Get("log_group"))
	assert.Equal(t, "web-stream", out.Get("log_stream"))
	assert.Equal(t, int64(30), out.Get("log_retention"))
	assert.Equal(t, "sigv4auth/syslog", out.Get("additional_auth"))
}

func TestProvisionerTranslator_ZeroRetention(t *testing.T) {
	tr := newProvisionerTranslator("syslog_0_default", "eu-west-1", "/logs", "stream", 0)
	cfg, err := tr.Translate(confmap.New())
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(cfg))
	assert.Equal(t, int64(0), out.Get("log_retention"))
}
