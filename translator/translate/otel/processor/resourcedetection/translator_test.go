// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcedetection

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

func TestTranslate(t *testing.T) {
	tt := NewTranslator(WithDataType(component.DataTypeTraces))
	testCases := map[string]struct {
		input   map[string]interface{}
		mode    string
		isECS   bool
		want    *confmap.Conf
		wantErr error
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
				"timeout":  "2s",
				"override": true,
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
				"timeout":  "2s",
				"override": true,
				"ec2": map[string]interface{}{
					"tags": []interface{}{"^kubernetes.io/cluster/.*$", "^aws:autoscaling:groupName"},
				},
			}),
		},
	}
	factory := resourcedetectionprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetMode(testCase.mode)
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
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(&wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}

func TestContainerInsightsJmx(t *testing.T) {
	transl := NewTranslatorWithName(common.PipelineNameContainerInsightsJmx).(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*resourcedetectionprocessor.Config)
	c := testutil.GetConf(t, filepath.Join("configs", "jmx_config.yaml"))
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("configs", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*resourcedetectionprocessor.Config)
	assert.True(t, ok)
	assert.Equal(t, len(actualCfg.Detectors), len(expectedCfg.Detectors))

}
