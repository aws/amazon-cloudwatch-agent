// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

func TestNewTranslatorsNilConf(t *testing.T) {
	translators := NewTranslators(nil)
	assert.Equal(t, 0, translators.Len())
}

func TestNewTranslatorsNoSyslogKey(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{}},
	})
	translators := NewTranslators(conf)
	assert.Equal(t, 0, translators.Len())
}

func TestNewTranslatorsSingleObjectNormalized(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": map[string]any{
				"listen_address": "tcp://0.0.0.0:514",
				"log_group_name": "/default",
			},
		}},
	})
	translators := NewTranslators(conf)
	// single listener with no rules → 1 default pipeline
	assert.Equal(t, 1, translators.Len())
	keys := translators.Keys()
	assert.Equal(t, "logs/syslog_0_default", keys[0].String())
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
						"match":          map[string]any{"facility": 4},
						"log_group_name": "/auth",
					},
				},
			},
		}},
	})
	translators := NewTranslators(conf)
	// 2 rules + 1 default = 3 pipelines
	assert.Equal(t, 3, translators.Len())
	keys := collections.MapSlice(translators.Keys(), pipeline.ID.String)
	assert.Equal(t, []string{
		"logs/syslog_0_rule_0",
		"logs/syslog_0_rule_1",
		"logs/syslog_0_default",
	}, keys)
}

func TestNewTranslatorsMultipleListeners(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": []any{
				map[string]any{
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/tcp/default",
					"routing": []any{
						map[string]any{
							"match":          map[string]any{"hostname": "web-*"},
							"log_group_name": "/tcp/web",
						},
					},
				},
				map[string]any{
					"listen_address": "udp://0.0.0.0:514",
					"log_group_name": "/udp/default",
				},
			},
		}},
	})
	translators := NewTranslators(conf)
	// TCP: 1 rule + 1 default = 2; UDP: 0 rules + 1 default = 1; total = 3
	assert.Equal(t, 3, translators.Len())
	keys := collections.MapSlice(translators.Keys(), pipeline.ID.String)
	assert.Equal(t, []string{
		"logs/syslog_0_rule_0",
		"logs/syslog_0_default",
		"logs/syslog_1_default",
	}, keys)
}

func TestPipelineTranslateComponents(t *testing.T) {
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
	translators := NewTranslators(conf)

	// Check rule pipeline components
	rulePipeline, ok := translators.Get(pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_rule_0"))
	require.True(t, ok)
	got, err := rulePipeline.Translate(conf)
	require.NoError(t, err)
	assert.Equal(t, []string{"syslog/tcp_0_0_0_0_514"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Equal(t, []string{"awssyslogrouter/syslog_0_rule_0", "batch/syslog_0_rule_0"}, collections.MapSlice(got.Processors.Keys(), component.ID.String))
	assert.Equal(t, []string{"awscloudwatchlogs/syslog_0_rule_0"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
	assert.Equal(t, []string{"agenthealth/logs", "agenthealth/statuscode"}, collections.MapSlice(got.Extensions.Keys(), component.ID.String))

	// Check default pipeline shares the same receiver
	defaultPipeline, ok := translators.Get(pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_default"))
	require.True(t, ok)
	got, err = defaultPipeline.Translate(conf)
	require.NoError(t, err)
	assert.Equal(t, []string{"syslog/tcp_0_0_0_0_514"}, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
	assert.Equal(t, []string{"awssyslogrouter/syslog_0_default", "batch/syslog_0_default"}, collections.MapSlice(got.Processors.Keys(), component.ID.String))
	assert.Equal(t, []string{"awscloudwatchlogs/syslog_0_default"}, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
}

func TestDeriveReceiverName(t *testing.T) {
	assert.Equal(t, "tcp_0_0_0_0_514", deriveReceiverName("tcp://0.0.0.0:514"))
	assert.Equal(t, "udp_127_0_0_1_6515", deriveReceiverName("udp://127.0.0.1:6515"))
}

func TestToMatchRule(t *testing.T) {
	rule := map[string]any{
		"match": map[string]any{
			"hostname": "web-*",
			"facility": 4,
			"app_name": "nginx",
		},
	}
	mr := toMatchRule(rule)
	assert.Equal(t, "web-*", mr.Hostname)
	assert.Equal(t, "nginx", mr.AppName)
	require.NotNil(t, mr.Facility)
	assert.Equal(t, 4, *mr.Facility)
}

func TestToMatchRuleNoMatch(t *testing.T) {
	rule := map[string]any{"log_group_name": "/test"}
	mr := toMatchRule(rule)
	assert.Empty(t, mr.Hostname)
	assert.Empty(t, mr.AppName)
	assert.Nil(t, mr.Facility)
}

func TestToMatchRuleInvalidFacilityType(t *testing.T) {
	rule := map[string]any{
		"match": map[string]any{
			"hostname": "db-*",
			"facility": "not_a_number",
		},
	}
	mr := toMatchRule(rule)
	assert.Equal(t, "db-*", mr.Hostname)
	assert.Nil(t, mr.Facility)
}

func TestToMatchRuleFacilityFloat64(t *testing.T) {
	rule := map[string]any{
		"match": map[string]any{
			"facility": float64(10),
		},
	}
	mr := toMatchRule(rule)
	require.NotNil(t, mr.Facility)
	assert.Equal(t, 10, *mr.Facility)
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
	translators := NewTranslators(conf)
	require.Equal(t, 1, translators.Len())

	pipelineTrans, ok := translators.Get(pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_default"))
	require.True(t, ok)

	got, err := pipelineTrans.Translate(conf)
	require.NoError(t, err)

	// Verify receiver is created for the TLS listener
	receiverIDs := collections.MapSlice(got.Receivers.Keys(), component.ID.String)
	assert.Equal(t, []string{"syslog/tcp_0_0_0_0_6514"}, receiverIDs)

	// Translate the receiver and verify TLS fields propagate to the OTel config
	receiverTrans, ok := got.Receivers.Get(component.NewIDWithName(component.MustNewType("syslog"), "tcp_0_0_0_0_6514"))
	require.True(t, ok)

	receiverCfg, err := receiverTrans.Translate(confmap.New())
	require.NoError(t, err)

	// Marshal the config back to a confmap and assert TLS values
	out := confmap.New()
	require.NoError(t, out.Marshal(receiverCfg))
	assert.Equal(t, "/etc/ssl/cert.pem", out.Get("tcp::tls::cert_file"))
	assert.Equal(t, "/etc/ssl/key.pem", out.Get("tcp::tls::key_file"))
	assert.Equal(t, "/etc/ssl/ca.pem", out.Get("tcp::tls::ca_file"))
	assert.Equal(t, "1.3", out.Get("tcp::tls::min_version"))
}

func TestNewTranslatorsWithClientCAFile(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"logs_collected": map[string]any{
			"syslog": map[string]any{
				"listen_address": "tcp://0.0.0.0:6514",
				"log_group_name": "/mtls/default",
				"tls": map[string]any{
					"cert_file":      "/etc/ssl/cert.pem",
					"key_file":       "/etc/ssl/key.pem",
					"client_ca_file": "/etc/ssl/client-ca.pem",
				},
			},
		}},
	})
	translators := NewTranslators(conf)
	require.Equal(t, 1, translators.Len())

	pipelineTrans, ok := translators.Get(pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_default"))
	require.True(t, ok)

	got, err := pipelineTrans.Translate(conf)
	require.NoError(t, err)

	// Translate the receiver and verify client_ca_file propagates
	receiverTrans, ok := got.Receivers.Get(component.NewIDWithName(component.MustNewType("syslog"), "tcp_0_0_0_0_6514"))
	require.True(t, ok)

	receiverCfg, err := receiverTrans.Translate(confmap.New())
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(receiverCfg))
	assert.Equal(t, "/etc/ssl/cert.pem", out.Get("tcp::tls::cert_file"))
	assert.Equal(t, "/etc/ssl/key.pem", out.Get("tcp::tls::key_file"))
	assert.Equal(t, "/etc/ssl/client-ca.pem", out.Get("tcp::tls::client_ca_file"))
}

func TestToFilters(t *testing.T) {
	testCases := []struct {
		name string
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
