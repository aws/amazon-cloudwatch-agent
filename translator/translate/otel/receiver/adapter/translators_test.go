// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	translatorconfig "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/config"
)

// TestFindReceiversInConfig confirms whether the given the agent json configuration
// will give the appropriate receivers in the agent yaml
func TestFindReceiversInConfig(t *testing.T) {
	type wantResult struct {
		cfgKey   string
		interval time.Duration
	}
	testCases := map[string]struct {
		input   map[string]interface{}
		os      string
		want    map[component.ID]wantResult
		wantErr error
	}{
		"WithEmptyConfig": {
			os:   "linux",
			want: map[component.ID]wantResult{},
		},
		"WithLinuxMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"collectd":   nil,
						"cpu":        nil,
						"ethtool":    nil,
						"nvidia_gpu": nil,
						"statsd":     nil,
						"procstat": []interface{}{
							map[string]interface{}{
								"exe":                         "amazon-cloudwatch-agent",
								"metrics_collection_interval": 15,
							},
							map[string]interface{}{
								"exe": "amazon-ssm-agent",
							},
						},
					},
				},
			},
			os: translatorconfig.OS_TYPE_LINUX,
			want: map[component.ID]wantResult{
				component.NewID("telegraf_socket_listener"):                {"metrics::metrics_collected::collectd", time.Minute},
				component.NewID("telegraf_cpu"):                            {"metrics::metrics_collected::cpu", time.Minute},
				component.NewID("telegraf_ethtool"):                        {"metrics::metrics_collected::ethtool", time.Minute},
				component.NewID("telegraf_nvidia_smi"):                     {"metrics::metrics_collected::nvidia_gpu", time.Minute},
				component.NewID("telegraf_statsd"):                         {"metrics::metrics_collected::statsd", 10 * time.Second},
				component.NewIDWithName("telegraf_procstat", "793254176"):  {"metrics::metrics_collected::procstat", time.Minute},
				component.NewIDWithName("telegraf_procstat", "3599690165"): {"metrics::metrics_collected::procstat", time.Minute},
			},
		},
		"WithWindowsMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"LogicalDisk": map[string]interface{}{
							"measurement":                 []string{"% Free Space"},
							"metrics_collection_interval": 10,
						},
						"Memory":       nil,
						"Paging File":  nil,
						"PhysicalDisk": nil,
						"nvidia_gpu":   nil,
						"procstat": []interface{}{
							map[string]interface{}{
								"exe":                         "amazon-cloudwatch-agent",
								"metrics_collection_interval": 5,
							},
							map[string]interface{}{
								"exe":                         "amazon-ssm-agent",
								"metrics_collection_interval": 15,
							},
						},
					},
				},
			},
			os: translatorconfig.OS_TYPE_WINDOWS,
			want: map[component.ID]wantResult{
				component.NewID("telegraf_nvidia_smi"):                              {"metrics::metrics_collected::nvidia_gpu", time.Minute},
				component.NewIDWithName("telegraf_procstat", "793254176"):           {"metrics::metrics_collected::procstat", time.Minute},
				component.NewIDWithName("telegraf_procstat", "3599690165"):          {"metrics::metrics_collected::procstat", time.Minute},
				component.NewIDWithName("telegraf_win_perf_counters", "4283769065"): {"metrics::metrics_collected::LogicalDisk", time.Minute},
				component.NewIDWithName("telegraf_win_perf_counters", "1492679118"): {"metrics::metrics_collected::Memory", time.Minute},
				component.NewIDWithName("telegraf_win_perf_counters", "3610923661"): {"metrics::metrics_collected::Paging File", time.Minute},
				component.NewIDWithName("telegraf_win_perf_counters", "3446270237"): {"metrics::metrics_collected::PhysicalDisk", time.Minute},
			},
		},
		"WithLogs": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf":           nil,
						"structuredlog": nil,
					},
					"logs_collected": map[string]interface{}{
						"files":          nil,
						"windows_events": nil,
					},
				},
			},
			os: translatorconfig.OS_TYPE_WINDOWS,
			want: map[component.ID]wantResult{
				component.NewID("telegraf_socket_listener"):   {"logs::metrics_collected::emf", time.Minute},
				component.NewID("telegraf_logfile"):           {"logs::logs_collected::files", time.Minute},
				component.NewID("telegraf_windows_event_log"): {"logs::logs_collected::windows_events", time.Minute},
			},
		},
		"WithThreeSocketListeners": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"collectd": nil,
					},
				},
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf":           nil,
						"structuredlog": nil,
					},
				},
			},
			os: translatorconfig.OS_TYPE_LINUX,
			want: map[component.ID]wantResult{
				component.NewID("telegraf_socket_listener"): {"metrics::metrics_collected::collectd", time.Minute},
			},
		},
		"WithInvalidOS": {
			input:   map[string]interface{}{},
			os:      "invalid",
			wantErr: errors.New("unsupported OS: invalid"),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := FindReceiversInConfig(conf, testCase.os)
			require.Equal(t, testCase.wantErr, err)
			require.Equal(t, len(testCase.want), len(got))
			for wantKey, wantValue := range testCase.want {
				gotTranslator, ok := got.Get(wantKey)
				require.True(t, ok)
				gotAdapterTranslator, ok := gotTranslator.(*translator)
				require.True(t, ok)
				require.Equal(t, wantValue.cfgKey, gotAdapterTranslator.cfgKey)
				require.Equal(t, wantValue.interval, gotAdapterTranslator.defaultMetricCollectionInterval)
			}
		})
	}
}
