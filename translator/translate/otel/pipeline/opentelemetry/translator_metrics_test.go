// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	gocontext "context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/tagutil"
)

// noTagsEC2Client returns no tags, used to deterministically exercise the "cluster name
// looked up but none found" path without any network calls.
type noTagsEC2Client struct{}

func (noTagsEC2Client) DescribeTags(gocontext.Context, *ec2.DescribeTagsInput, ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	return &ec2.DescribeTagsOutput{}, nil
}

func TestBaseMetricsTranslator(t *testing.T) {
	tt := NewBaseMetricsTranslator()
	assert.EqualValues(t, "metrics/opentelemetry", tt.ID().String())

	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr error
	}{
		"WithNilConf": {
			input:   nil,
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: strings.Join(otelMetricsKeys, " or ")},
		},
		"WithoutCollectKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: strings.Join(otelMetricsKeys, " or ")},
		},
		"WithCollectKeyButNoMetricsSource": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{},
				},
			},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: strings.Join(otelMetricsKeys, " or ")},
		},
		"WithWindowsEventsOnlyNoMetrics": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"windows_events": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{"event_name": "System"},
							},
						},
					},
				},
			},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: strings.Join(otelMetricsKeys, " or ")},
		},
		"WithOtlpKey": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			agent.Global_Config.Region = "us-west-2"
			var conf *confmap.Conf
			if tc.input != nil {
				conf = confmap.NewFromStringMap(tc.input)
			}
			got, err := tt.Translate(conf)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, 1, got.Receivers.Len())
				assert.Equal(t, 4, got.Processors.Len())
				assert.Equal(t, "resourcedetection/opentelemetry", got.Processors.Keys()[0].String())
				assert.Equal(t, "transform/identity", got.Processors.Keys()[1].String())
				assert.Equal(t, "awsattributelimit/opentelemetry_metrics", got.Processors.Keys()[2].String())
				assert.Equal(t, "batch/opentelemetry_metrics", got.Processors.Keys()[3].String())
				assert.Equal(t, 1, got.Exporters.Len())
				assert.Equal(t, 2, got.Extensions.Len())
				assert.Equal(t, 1, got.Connectors.Len())
				assert.Equal(t, "forward/opentelemetry", got.Receivers.Keys()[0].String())
				assert.Equal(t, "otlp_http/metrics", got.Exporters.Keys()[0].String())
				assert.Equal(t, "sigv4auth/monitoring", got.Extensions.Keys()[0].String())
				assert.Equal(t, "forward/opentelemetry", got.Connectors.Keys()[0].String())
			}
		})
	}
}

func TestBaseMetricsTranslatorResourceAttributes(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	tt := NewBaseMetricsTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"resource_attributes": map[string]interface{}{
				"team": "cloudwatch",
			},
			"collect": map[string]interface{}{
				"otlp": map[string]interface{}{},
			},
		},
	})
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 5, got.Processors.Len())
	assert.Equal(t, "resource/opentelemetry", got.Processors.Keys()[0].String())
	assert.Equal(t, "resourcedetection/opentelemetry", got.Processors.Keys()[1].String())
	assert.Equal(t, "transform/identity", got.Processors.Keys()[2].String())
	assert.Equal(t, "awsattributelimit/opentelemetry_metrics", got.Processors.Keys()[3].String())
	assert.Equal(t, "batch/opentelemetry_metrics", got.Processors.Keys()[4].String())
}

func TestBaseMetricsTranslatorEmptyRegion(t *testing.T) {
	agent.Global_Config.Region = ""
	tt := NewBaseMetricsTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"otlp": map[string]interface{}{},
			},
		},
	})
	got, err := tt.Translate(conf)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "region is required")
}

func TestBaseMetricsTranslatorClusterName(t *testing.T) {
	agent.Global_Config.Region = "us-east-1"
	// Cluster name is only applied in a Kubernetes environment.
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)
	t.Cleanup(func() { context.CurrentContext().SetKubernetesMode("") })
	tt := NewBaseMetricsTranslator()

	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"host_metrics": map[string]interface{}{},
			},
		},
	})

	got, err := tt.Translate(conf)
	require.NoError(t, err)

	// Verify set_cluster_name processor is present
	keys := make([]string, 0, got.Processors.Len())
	for _, k := range got.Processors.Keys() {
		keys = append(keys, k.String())
	}
	assert.Contains(t, keys, "transform/set_cluster_name")
}

// TestClusterNameSkippedNonK8s verifies the cluster name is gated on Kubernetes mode
func TestClusterNameSkippedNonK8s(t *testing.T) {
	agent.Global_Config.Region = "us-east-1"
	context.CurrentContext().SetKubernetesMode("") // non-Kubernetes (EC2 host)
	t.Cleanup(func() { context.CurrentContext().SetKubernetesMode("") })
	tt := NewBaseMetricsTranslator()

	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"cluster_name": "test-cluster",
			"collect": map[string]interface{}{
				"host_metrics": map[string]interface{}{},
			},
		},
	})

	got, err := tt.Translate(conf)
	require.NoError(t, err)

	keys := make([]string, 0, got.Processors.Len())
	for _, k := range got.Processors.Keys() {
		keys = append(keys, k.String())
	}
	assert.NotContains(t, keys, "transform/set_cluster_name")
}

func TestBaseMetricsTranslatorNoClusterName(t *testing.T) {
	agent.Global_Config.Region = "us-east-1"
	// Force the ec2 tag lookup with no network to return no cluster name
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)
	t.Cleanup(func() { context.CurrentContext().SetKubernetesMode("") })
	tagutil.SetEC2APIProviderForTesting(func() ec2.DescribeTagsAPIClient {
		return noTagsEC2Client{}
	})
	t.Cleanup(tagutil.ResetEC2APIProvider)
	t.Cleanup(tagutil.ResetTagsCache)

	tt := NewBaseMetricsTranslator()

	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"host_metrics": map[string]interface{}{},
			},
		},
	})

	got, err := tt.Translate(conf)
	require.NoError(t, err)

	keys := make([]string, 0, got.Processors.Len())
	for _, k := range got.Processors.Keys() {
		keys = append(keys, k.String())
	}
	assert.NotContains(t, keys, "transform/set_cluster_name")
}

func TestBaseMetricsBatch1000In10s(t *testing.T) {
	prevRegion := agent.Global_Config.Region
	agent.Global_Config.Region = "us-west-2"
	t.Cleanup(func() { agent.Global_Config.Region = prevRegion })
	tt := NewBaseMetricsTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{"otlp": map[string]interface{}{}},
		},
	})
	got, err := tt.Translate(conf)
	require.NoError(t, err)

	var batchT common.ComponentTranslator
	for _, id := range got.Processors.Keys() {
		if id.String() == "batch/opentelemetry_metrics" {
			batchT, _ = got.Processors.Get(id)
		}
	}
	require.NotNil(t, batchT)
	cfg, err := batchT.Translate(conf)
	require.NoError(t, err)
	bcfg := cfg.(*batchprocessor.Config)
	assert.Equal(t, 10*time.Second, bcfg.Timeout)
	assert.EqualValues(t, 1000, bcfg.SendBatchSize)
	assert.EqualValues(t, 1000, bcfg.SendBatchMaxSize)
}
