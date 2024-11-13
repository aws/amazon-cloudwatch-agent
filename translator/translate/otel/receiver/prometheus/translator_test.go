// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"
	promcommon "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		input   map[string]any
		wantID  string
		want    *prometheusreceiver.Config
		wantErr error
	}{
		"WithOtelConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			wantID: "prometheus",
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: promconfig.GlobalConfig{
						ScrapeInterval:     model.Duration(1 * time.Minute),
						ScrapeTimeout:      model.Duration(30 * time.Second),
						ScrapeProtocols:    promconfig.DefaultScrapeProtocols,
						EvaluationInterval: model.Duration(1 * time.Minute),
					},
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:    model.Duration(1 * time.Minute),
							ScrapeTimeout:     model.Duration(30 * time.Second),
							JobName:           "prometheus_test_job",
							HonorTimestamps:   true,
							ScrapeProtocols:   promconfig.DefaultScrapeProtocols,
							MetricsPath:       "/metrics",
							Scheme:            "http",
							EnableCompression: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									&targetgroup.Group{
										Targets: []model.LabelSet{
											{
												model.AddressLabel: "localhost:8000",
											},
										},
										Labels: map[model.LabelName]model.LabelValue{
											"label1": "test1",
										},
										Source: "0",
									},
								},
							},
							HTTPClientConfig: promcommon.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
							},
						},
					},
				},
				TargetAllocator: &targetallocator.Config{
					CollectorID: "col-1234",
					ClientConfig: confighttp.ClientConfig{
						TLSSetting: configtls.ClientConfig{
							Config: configtls.Config{
								CAFile:         defaultTlsCaPath,
								ReloadInterval: 10 * time.Second,
							},
						},
					},
				},
			},
		},
		"WithPromConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "config_prom.json")),
			wantID: "prometheus",
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: promconfig.GlobalConfig{
						ScrapeInterval:     model.Duration(1 * time.Minute),
						ScrapeTimeout:      model.Duration(30 * time.Second),
						ScrapeProtocols:    promconfig.DefaultScrapeProtocols,
						EvaluationInterval: model.Duration(1 * time.Minute),
					},
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:    model.Duration(1 * time.Minute),
							ScrapeTimeout:     model.Duration(30 * time.Second),
							JobName:           "prometheus_test_job",
							HonorTimestamps:   true,
							ScrapeProtocols:   promconfig.DefaultScrapeProtocols,
							MetricsPath:       "/metrics",
							Scheme:            "http",
							EnableCompression: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									&targetgroup.Group{
										Targets: []model.LabelSet{
											{
												model.AddressLabel: "localhost:8000",
											},
										},
										Labels: map[model.LabelName]model.LabelValue{
											"label1": "test1",
										},
										Source: "0",
									},
								},
							},
							HTTPClientConfig: promcommon.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
							},
						},
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator()
			assert.EqualValues(t, testCase.wantID, tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*prometheusreceiver.Config)
				require.True(t, ok)

				assert.NoError(t, err)
				assert.Equal(t, testCase.want.PrometheusConfig.ScrapeConfigs, gotCfg.PrometheusConfig.ScrapeConfigs)
				assert.Equal(t, testCase.want.PrometheusConfig.GlobalConfig, gotCfg.PrometheusConfig.GlobalConfig)
				assert.Equal(t, testCase.want.TargetAllocator, gotCfg.TargetAllocator)
			}
		})
	}
}
