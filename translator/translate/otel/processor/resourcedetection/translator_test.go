// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcedetection

import (
	"embed"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"

	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

// resourceDetectionConfigsFS mirrors the set of configs embedded by translator.go so the
// guard test below iterates exactly the configs shipped by the agent.
//
//go:embed configs/*.yaml
var resourceDetectionConfigsFS embed.FS

func TestTranslate(t *testing.T) {
	tt := NewTranslator(WithSignal(pipeline.SignalTraces))
	testCases := map[string]struct {
		input          map[string]interface{}
		mode           string
		kubernetesMode string
		isECS          bool
		want           *confmap.Conf
		wantErr        error
	}{
		"WithAppSignalsEnabledOnECS": {
			mode:  translatorconfig.ModeEC2,
			isECS: true,
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"detectors": []interface{}{
					"env",
					"ecs",
					"ec2",
				},
				"timeout":                "2s",
				"override":               true,
				"ignore_detector_errors": true,
				"ec2": map[string]interface{}{
					"tags": []interface{}{"^aws:autoscaling:groupName"},
				},
				"ecs": map[string]interface{}{
					"resource_attributes": map[string]interface{}{
						"aws.ecs.cluster.arn": map[string]interface{}{
							"enabled": true,
						},
						"aws.ecs.launchtype": map[string]interface{}{
							"enabled": true,
						},
						"aws.ecs.task.arn": map[string]interface{}{
							"enabled": false,
						},
						"aws.ecs.task.family": map[string]interface{}{
							"enabled": false,
						},
						"aws.ecs.task.id": map[string]interface{}{
							"enabled": false,
						},
						"aws.ecs.task.revision": map[string]interface{}{
							"enabled": false,
						},
						"aws.log.group.arns": map[string]interface{}{
							"enabled": false,
						},
						"aws.log.group.names": map[string]interface{}{
							"enabled": false,
						},
						"aws.log.stream.arns": map[string]interface{}{
							"enabled": false,
						},
						"aws.log.stream.names": map[string]interface{}{
							"enabled": false,
						},
						"cloud.account.id": map[string]interface{}{
							"enabled": true,
						},
						"cloud.availability_zone": map[string]interface{}{
							"enabled": true,
						},
						"cloud.platform": map[string]interface{}{
							"enabled": true,
						},
						"cloud.provider": map[string]interface{}{
							"enabled": true,
						},
						"cloud.region": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		"WithAppSignalsEnabledOnEC2": {
			mode: translatorconfig.ModeEC2,
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"detectors": []interface{}{
					"eks",
					"env",
					"ec2",
				},
				"timeout":                "2s",
				"override":               true,
				"ignore_detector_errors": true,
				"eks": map[string]interface{}{
					"node_from_env_var": "HOST_NAME",
				},
				"ec2": map[string]interface{}{
					"tags": []interface{}{"^kubernetes.io/cluster/.*$", "^aws:autoscaling:groupName"},
				},
			}),
		},
		"WithAppSignalsEnabledOnAzureVM": {
			mode: translatorconfig.ModeAzureVM,
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"detectors": []interface{}{
					"env",
					"azure",
				},
				"timeout":                "2s",
				"override":               true,
				"ignore_detector_errors": true,
			}),
		},
		"WithAppSignalsEnabledOnAKS": {
			kubernetesMode: translatorconfig.ModeAKS,
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"detectors": []interface{}{
					"env",
					"aks",
					"azure",
				},
				"timeout":                "2s",
				"override":               true,
				"ignore_detector_errors": true,
			}),
		},
		"WithAppSignalsEnabledOnGCE": {
			mode: translatorconfig.ModeGCE,
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"detectors": []interface{}{
					"env",
					"gcp",
				},
				"timeout":  "2s",
				"override": true,
			}),
		},
		"WithAppSignalsEnabledOnGKE": {
			kubernetesMode: translatorconfig.ModeGKE,
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"detectors": []interface{}{
					"env",
					"gcp",
				},
				"timeout":  "2s",
				"override": true,
			}),
		},
	}
	factory := resourcedetectionprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Reset so kubernetesMode/mode does not bleed across cases (map iteration is non-deterministic).
			context.ResetContext()
			if testCase.kubernetesMode != "" {
				context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			} else {
				context.CurrentContext().SetMode(testCase.mode)
			}
			if testCase.isECS {
				ecsutil.GetECSUtilSingleton().Region = "test-region"
			} else {
				ecsutil.GetECSUtilSingleton().Region = ""
			}
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*resourcedetectionprocessor.Config)
				assert.NotNil(t, gotCfg.MiddlewareID)
				gotCfg.MiddlewareID = nil
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(&wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}

func TestTranslate_OpenTelemetryKey_NoMiddleware(t *testing.T) {
	tt := NewTranslator(WithName("opentelemetry"))
	context.ResetContext()
	context.CurrentContext().SetMode(translatorconfig.ModeEC2)
	ecsutil.GetECSUtilSingleton().Region = ""
	conf := confmap.NewFromStringMap(map[string]interface{}{})
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)
	gotCfg := got.(*resourcedetectionprocessor.Config)
	assert.Nil(t, gotCfg.MiddlewareID)
}

func TestTranslate_NonOpenTelemetryKey_HasMiddleware(t *testing.T) {
	tt := NewTranslator(WithSignal(pipeline.SignalTraces))
	context.CurrentContext().SetMode(translatorconfig.ModeEC2)
	ecsutil.GetECSUtilSingleton().Region = ""
	conf := confmap.NewFromStringMap(map[string]interface{}{})
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)
	gotCfg := got.(*resourcedetectionprocessor.Config)
	assert.NotNil(t, gotCfg.MiddlewareID)
}

// TestAllEmbeddedConfigsIgnoreDetectorErrors is a process guard: v0.150 removed the
// propagateerrors gate, so a detector failure aborts collector startup unless
// ignore_detector_errors is set. Every embedded resource detection config must therefore
// opt into log-and-continue. This test fails if any current or future config omits it.
func TestAllEmbeddedConfigsIgnoreDetectorErrors(t *testing.T) {
	entries, err := resourceDetectionConfigsFS.ReadDir("configs")
	require.NoError(t, err)

	var checked int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		name := entry.Name()
		data, err := resourceDetectionConfigsFS.ReadFile("configs/" + name)
		require.NoErrorf(t, err, "reading embedded config %q", name)

		var cfg map[string]interface{}
		require.NoErrorf(t, yaml.Unmarshal(data, &cfg), "parsing embedded config %q", name)

		val, ok := cfg["ignore_detector_errors"]
		require.Truef(t, ok, "embedded config %q must declare ignore_detector_errors", name)
		assert.Equalf(t, true, val, "embedded config %q must set ignore_detector_errors: true", name)
		checked++
	}
	require.Positive(t, checked, "expected to check at least one embedded resource detection config")
}
