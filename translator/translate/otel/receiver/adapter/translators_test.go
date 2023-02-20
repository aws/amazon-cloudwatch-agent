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
)

func TestFindReceiversInConfig(t *testing.T) {
	type wantResult struct {
		cfgKey   string
		interval time.Duration
	}
	testCases := map[string]struct {
		input   map[string]interface{}
		os      string
		want    map[component.Type]wantResult
		wantErr error
	}{
		"WithEmptyConfig": {
			os:   "linux",
			want: map[component.Type]wantResult{},
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
					},
				},
			},
			os: "linux",
			want: map[component.Type]wantResult{
				"telegraf_socket_listener": {"metrics::metrics_collected::collectd", time.Minute},
				"telegraf_cpu":             {"metrics::metrics_collected::cpu", time.Minute},
				"telegraf_ethtool":         {"metrics::metrics_collected::ethtool", time.Minute},
				"telegraf_nvidia_smi":      {"metrics::metrics_collected::nvidia_gpu", time.Minute},
				"telegraf_statsd":          {"metrics::metrics_collected::statsd", 10 * time.Second},
			},
		},
		"WithWindowsMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"LogicalDisk":  nil,
						"Memory":       nil,
						"Paging File":  nil,
						"PhysicalDisk": nil,
						"nvidia_gpu":   nil,
					},
				},
			},
			os: "windows",
			want: map[component.Type]wantResult{
				"telegraf_nvidia_smi":        {"metrics::metrics_collected::nvidia_gpu", time.Minute},
				"telegraf_win_perf_counters": {"metrics", time.Minute},
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
			os: "windows",
			want: map[component.Type]wantResult{
				"telegraf_socket_listener":   {"logs::metrics_collected::emf", time.Minute},
				"telegraf_logfile":           {"logs::logs_collected::files", time.Minute},
				"telegraf_windows_event_log": {"logs::logs_collected::windows_events", time.Minute},
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
			os: "linux",
			want: map[component.Type]wantResult{
				"telegraf_socket_listener": {"metrics::metrics_collected::collectd", time.Minute},
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
