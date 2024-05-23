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

	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
)

// TestFindReceiversInConfig confirms whether the given the agent json configuration
// will give the appropriate receivers in the agent yaml
func TestFindReceiversInConfig(t *testing.T) {
	telegrafSocketListenerType, _ := component.NewType("telegraf_socket_listener")
	telegrafCPUType, _ := component.NewType("telegraf_cpu")
	telegrafEthtoolType, _ := component.NewType("telegraf_ethtool")
	telegrafNvidiaSmiType, _ := component.NewType("telegraf_nvidia_smi")
	telegrafStatsdType, _ := component.NewType("telegraf_statsd")
	telegrafProcstatType, _ := component.NewType("telegraf_procstat")
	telegrafWinPerfCountersType, _ := component.NewType("telegraf_win_perf_counters")
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
						"collectd":   map[string]interface{}{},
						"cpu":        map[string]interface{}{},
						"ethtool":    map[string]interface{}{},
						"nvidia_gpu": map[string]interface{}{},
						"statsd":     map[string]interface{}{},
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
				component.NewID(telegrafSocketListenerType):                 {"metrics::metrics_collected::collectd", time.Minute},
				component.NewID(telegrafCPUType):                            {"metrics::metrics_collected::cpu", time.Minute},
				component.NewID(telegrafEthtoolType):                        {"metrics::metrics_collected::ethtool", time.Minute},
				component.NewID(telegrafNvidiaSmiType):                      {"metrics::metrics_collected::nvidia_gpu", time.Minute},
				component.NewID(telegrafStatsdType):                         {"metrics::metrics_collected::statsd", 10 * time.Second},
				component.NewIDWithName(telegrafProcstatType, "793254176"):  {"metrics::metrics_collected::procstat", time.Minute},
				component.NewIDWithName(telegrafProcstatType, "3599690165"): {"metrics::metrics_collected::procstat", time.Minute},
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
						"Memory":       map[string]interface{}{},
						"Paging File":  map[string]interface{}{},
						"PhysicalDisk": map[string]interface{}{},
						"nvidia_gpu":   map[string]interface{}{},
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
				component.NewID(telegrafNvidiaSmiType):                             {"metrics::metrics_collected::nvidia_gpu", time.Minute},
				component.NewIDWithName(telegrafProcstatType, "793254176"):         {"metrics::metrics_collected::procstat", time.Minute},
				component.NewIDWithName(telegrafProcstatType, "3599690165"):        {"metrics::metrics_collected::procstat", time.Minute},
				component.NewIDWithName(telegrafWinPerfCountersType, "4283769065"): {"metrics::metrics_collected::LogicalDisk", time.Minute},
				component.NewIDWithName(telegrafWinPerfCountersType, "1492679118"): {"metrics::metrics_collected::Memory", time.Minute},
				component.NewIDWithName(telegrafWinPerfCountersType, "3610923661"): {"metrics::metrics_collected::Paging File", time.Minute},
				component.NewIDWithName(telegrafWinPerfCountersType, "3446270237"): {"metrics::metrics_collected::PhysicalDisk", time.Minute},
			},
		},
		"WithLogs": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf":           map[string]interface{}{},
						"structuredlog": map[string]interface{}{},
					},
					"logs_collected": map[string]interface{}{
						"files":          map[string]interface{}{},
						"windows_events": map[string]interface{}{},
					},
				},
			},
			os:   translatorconfig.OS_TYPE_WINDOWS,
			want: map[component.ID]wantResult{},
		},
		"WithNoSocketListener": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf":           map[string]interface{}{},
						"structuredlog": map[string]interface{}{},
					},
				},
			},
			os:   translatorconfig.OS_TYPE_LINUX,
			want: map[component.ID]wantResult{},
		},
		"WithOneSocketListener": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"collectd": map[string]interface{}{},
					},
				},
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf":           map[string]interface{}{},
						"structuredlog": map[string]interface{}{},
					},
				},
			},
			os: translatorconfig.OS_TYPE_LINUX,
			want: map[component.ID]wantResult{
				component.NewID(telegrafSocketListenerType): {"metrics::metrics_collected::collectd", time.Minute},
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
			require.Equal(t, len(testCase.want), got.Len())
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
