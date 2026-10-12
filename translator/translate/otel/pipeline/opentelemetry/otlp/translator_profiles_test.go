// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

var otlpSectionConf = map[string]interface{}{
	"opentelemetry": map[string]interface{}{
		"collect": map[string]interface{}{
			"otlp": map[string]interface{}{},
		},
	},
}

var otlpSectionWithClusterNameConf = map[string]interface{}{
	"opentelemetry": map[string]interface{}{
		"cluster_name": "test-cluster",
		"collect": map[string]interface{}{
			"otlp": map[string]interface{}{},
		},
	},
}

func TestProfilesPipelineTranslator(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := &profilesPipelineTranslator{}
	assert.EqualValues(t, "profiles/otlp", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		region  string
		k8sMode string
		ecs     bool
		want    *want
		wantErr error
	}{
		"WithoutOtlpKey": {
			input:   map[string]interface{}{},
			region:  "us-east-1",
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: otlpKey},
		},
		"WithoutRegion": {
			input:   otlpSectionConf,
			wantErr: fmt.Errorf("region is required for %s profiles pipeline", common.OpenTelemetryKey),
		},
		"WithDefaults": {
			input:  otlpSectionConf,
			region: "us-east-1",
			want: &want{
				receivers:  []string{"otlp/grpc_127_0_0_1_4317", "otlp/http_127_0_0_1_4318"},
				processors: []string{"transform/otlp_scope", "resourcedetection/opentelemetry", "transform/identity", "resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/opentelemetry_profiles"},
			},
		},
		"WithEndpoints": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{
							"grpc_endpoint": "127.0.0.1:5317",
							"http_endpoint": "127.0.0.1:5318",
						},
					},
				},
			},
			region: "us-east-1",
			want: &want{
				receivers:  []string{"otlp/grpc_127_0_0_1_5317", "otlp/http_127_0_0_1_5318"},
				processors: []string{"transform/otlp_scope", "resourcedetection/opentelemetry", "transform/identity", "resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/opentelemetry_profiles"},
			},
		},
		"WithResourceAttributes": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"resource_attributes": map[string]interface{}{"team": "cloudwatch"},
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			region: "us-east-1",
			want: &want{
				receivers:  []string{"otlp/grpc_127_0_0_1_4317", "otlp/http_127_0_0_1_4318"},
				processors: []string{"transform/otlp_scope", "resource/opentelemetry", "resourcedetection/opentelemetry", "transform/identity", "resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/opentelemetry_profiles"},
			},
		},
		"WithECS": {
			input:  otlpSectionConf,
			region: "us-east-1",
			ecs:    true,
			want: &want{
				receivers:  []string{"otlp/grpc_127_0_0_1_4317", "otlp/http_127_0_0_1_4318"},
				processors: []string{"transform/otlp_scope", "resourcedetection/profiles", "transform/identity", "resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/opentelemetry_profiles", "agenthealth/statuscode"},
			},
		},
		"WithKubernetesAndClusterName": {
			input:   otlpSectionWithClusterNameConf,
			region:  "us-east-1",
			k8sMode: config.ModeEKS,
			want: &want{
				receivers:  []string{"otlp/grpc_127_0_0_1_4317", "otlp/http_127_0_0_1_4318"},
				processors: []string{"transform/otlp_scope", "resourcedetection/opentelemetry", "k8s_attributes/profiles", "transform/set_cluster_name", "transform/identity", "resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/opentelemetry_profiles"},
			},
		},
		"WithKubernetesResourceAttributesAndClusterName": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"cluster_name":        "test-cluster",
					"resource_attributes": map[string]interface{}{"team": "cloudwatch"},
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			region:  "us-east-1",
			k8sMode: config.ModeEKS,
			want: &want{
				receivers:  []string{"otlp/grpc_127_0_0_1_4317", "otlp/http_127_0_0_1_4318"},
				processors: []string{"transform/otlp_scope", "resource/opentelemetry", "resourcedetection/opentelemetry", "k8s_attributes/profiles", "transform/set_cluster_name", "transform/identity", "resource/profiles"},
				exporters:  []string{"otlp_http/profiles"},
				extensions: []string{"sigv4auth/monitoring", "agenthealth/opentelemetry_profiles"},
			},
		},
		"WithKubernetesInvalidClusterName": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"cluster_name": "bad cluster name!",
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			},
			region:  "us-east-1",
			k8sMode: config.ModeEKS,
			wantErr: common.ValidateClusterName("bad cluster name!"),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlpreceiver.ClearConfigCache)
			resetGlobalConfig(t, testCase.region)
			setKubernetesMode(t, testCase.k8sMode)
			setECS(t, testCase.ecs)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
				assertProfilesInvariants(t, got)
			}
		})
	}
}

func TestProfilesExporterEndpoint(t *testing.T) {
	testCases := map[string]struct {
		region string
		want   string
	}{
		"WithRegion": {
			region: "us-east-1",
			want:   "https://monitoring.us-east-1.amazonaws.com/v1development/profiles",
		},
		"WithChinaRegion": {
			region: "cn-north-1",
			want:   "https://monitoring.cn-north-1.amazonaws.com.cn/v1development/profiles",
		},
		"WithGovCloudRegion": {
			region: "us-gov-west-1",
			want:   "https://monitoring.us-gov-west-1.amazonaws.com/v1development/profiles",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlpreceiver.ClearConfigCache)
			resetGlobalConfig(t, testCase.region)
			conf := confmap.NewFromStringMap(otlpSectionConf)
			got, err := (&profilesPipelineTranslator{}).Translate(conf)
			require.NoError(t, err)
			exporterTranslator, ok := got.Exporters.Get(component.MustNewIDWithName("otlp_http", "profiles"))
			require.True(t, ok)
			cfg, err := exporterTranslator.Translate(conf)
			require.NoError(t, err)
			exporterCfg, ok := cfg.(*otlphttpexporter.Config)
			require.True(t, ok)
			assert.Equal(t, testCase.want, exporterCfg.ProfilesEndpoint)
			assert.EqualValues(t, "gzip", exporterCfg.ClientConfig.Compression)
			require.True(t, exporterCfg.ClientConfig.Auth.HasValue())
			assert.Equal(t, "agenthealth/opentelemetry_profiles", exporterCfg.ClientConfig.Auth.Get().AuthenticatorID.String())
		})
	}
}

func TestProfilesServiceNameFallbackStamped(t *testing.T) {
	t.Cleanup(otlpreceiver.ClearConfigCache)
	resetGlobalConfig(t, "us-east-1")
	cfg := profilesResourceProcessorConfig(t, confmap.NewFromStringMap(otlpSectionConf))
	require.Len(t, cfg.AttributesActions, 1)
	assert.Equal(t, "service.name", cfg.AttributesActions[0].Key)
	assert.EqualValues(t, "insert", cfg.AttributesActions[0].Action)
	assert.Equal(t, "unknown_service", cfg.AttributesActions[0].Value)
}

func TestProfilesServiceNameInferredOnEC2(t *testing.T) {
	t.Cleanup(otlpreceiver.ClearConfigCache)
	resetGlobalConfig(t, "us-east-1")
	stubEC2ServiceName(t, "my-ec2-service")
	cfg := profilesResourceProcessorConfig(t, confmap.NewFromStringMap(otlpSectionConf))
	require.Len(t, cfg.AttributesActions, 2)
	assert.Equal(t, "service.name", cfg.AttributesActions[0].Key)
	assert.EqualValues(t, "insert", cfg.AttributesActions[0].Action)
	assert.Equal(t, "my-ec2-service", cfg.AttributesActions[0].Value)
	assert.Equal(t, "service.name", cfg.AttributesActions[1].Key)
	assert.EqualValues(t, "insert", cfg.AttributesActions[1].Action)
	assert.Equal(t, "unknown_service", cfg.AttributesActions[1].Value)
}

func TestProfilesServiceNameNotInferredOffEC2(t *testing.T) {
	testCases := map[string]struct {
		mode    string
		k8sMode string
		ecs     bool
	}{
		"OnPrem":     {mode: config.ModeOnPrem},
		"Kubernetes": {mode: config.ModeEC2, k8sMode: config.ModeEKS},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlpreceiver.ClearConfigCache)
			resetGlobalConfig(t, "us-east-1")
			stubEC2ServiceName(t, "should-not-be-used")
			context.ResetContext()
			t.Cleanup(context.ResetContext)
			context.CurrentContext().SetMode(testCase.mode)
			setKubernetesMode(t, testCase.k8sMode)
			setECS(t, testCase.ecs)
			cfg := profilesResourceProcessorConfig(t, confmap.NewFromStringMap(otlpSectionConf))
			require.Len(t, cfg.AttributesActions, 1)
			assert.Equal(t, "unknown_service", cfg.AttributesActions[0].Value)
		})
	}
}

func TestProfilesServiceNameInferredOnECS(t *testing.T) {
	t.Cleanup(otlpreceiver.ClearConfigCache)
	resetGlobalConfig(t, "us-east-1")
	stubEC2ServiceName(t, "should-not-be-used")
	setECS(t, true)
	conf := confmap.NewFromStringMap(otlpSectionConf)
	got, err := (&profilesPipelineTranslator{}).Translate(conf)
	require.NoError(t, err)
	detectionTranslator, ok := got.Processors.Get(component.MustNewIDWithName("resourcedetection", "profiles"))
	require.True(t, ok)
	detectionCfg, err := detectionTranslator.Translate(conf)
	require.NoError(t, err)
	detection, ok := detectionCfg.(*resourcedetectionprocessor.Config)
	require.True(t, ok)
	assert.Equal(t, []string{"env", "ecs", "ec2"}, detection.Detectors)
	assert.True(t, detection.DetectorConfig.ECSConfig.ResourceAttributes.AwsEcsTaskFamily.Enabled)
	require.NotNil(t, detection.MiddlewareID)
	assert.Equal(t, "agenthealth/statuscode", detection.MiddlewareID.String())

	cfg := profilesResourceProcessorConfig(t, conf)
	require.Len(t, cfg.AttributesActions, 2)
	assert.EqualValues(t, "insert", cfg.AttributesActions[0].Action)
	assert.Equal(t, "service.name", cfg.AttributesActions[0].Key)
	assert.Equal(t, "aws.ecs.task.family", cfg.AttributesActions[0].FromAttribute)
	assert.EqualValues(t, "insert", cfg.AttributesActions[1].Action)
	assert.Equal(t, "unknown_service", cfg.AttributesActions[1].Value)
}

func TestProfilesIdentityTransformCarriesProfileStatements(t *testing.T) {
	testCases := map[string]struct {
		k8sMode string
	}{
		"EC2": {},
		"EKS": {k8sMode: config.ModeEKS},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlpreceiver.ClearConfigCache)
			resetGlobalConfig(t, "us-east-1")
			setKubernetesMode(t, testCase.k8sMode)
			conf := confmap.NewFromStringMap(otlpSectionWithClusterNameConf)
			got, err := (&profilesPipelineTranslator{}).Translate(conf)
			require.NoError(t, err)
			identity, ok := got.Processors.Get(component.MustNewIDWithName("transform", "identity"))
			require.True(t, ok)
			cfg, err := identity.Translate(conf)
			require.NoError(t, err)
			identityCfg, ok := cfg.(*transformprocessor.Config)
			require.True(t, ok)
			require.Len(t, identityCfg.ProfileStatements, 1)
			assert.Equal(t, "resource", string(identityCfg.ProfileStatements[0].Context))
			statements := identityCfg.ProfileStatements[0].Statements
			assert.True(t, containsSubstring(statements, `set(resource.attributes["deployment.environment.name"]`))
			assert.True(t, containsSubstring(statements, `set(resource.attributes["cloud.resource_id"]`))
			assert.False(t, containsSubstring(statements, `set(resource.attributes["service.name"], "unknown_service")`))
			assert.False(t, containsSubstring(statements, `replace_pattern(resource.attributes["service.name"]`))
			if testCase.k8sMode == "" {
				assert.False(t, containsSubstring(statements, `resource.attributes["service.name"]`))
			} else {
				assert.True(t, containsSubstring(statements, `set(resource.attributes["service.name"], resource.attributes["resource.opentelemetry.io/service.name"]) where resource.attributes["service.name"] == nil`))
				assert.True(t, containsSubstring(statements, `set(resource.attributes["service.name"], resource.attributes["k8s.pod.name"]) where resource.attributes["service.name"] == nil`))
				assert.True(t, containsSubstring(statements, `set(resource.attributes["service.namespace"], resource.attributes["k8s.namespace.name"]) where resource.attributes["service.namespace"] == nil`))
				assert.True(t, containsSubstring(statements, `delete_key(resource.attributes, "resource.opentelemetry.io/service.name")`))
				assert.True(t, containsSubstring(statements, `delete_key(resource.attributes, "app.kubernetes.io/name")`))
			}
			assert.True(t, containsSubstring(identityCfg.MetricStatements[0].Statements, `set(resource.attributes["service.name"], "unknown_service")`))
		})
	}
}

func TestProfilesKubernetesProcessors(t *testing.T) {
	t.Cleanup(otlpreceiver.ClearConfigCache)
	resetGlobalConfig(t, "us-east-1")
	setKubernetesMode(t, config.ModeEKS)
	t.Setenv("K8S_NODE_NAME", "node_name_from_env")
	conf := confmap.NewFromStringMap(otlpSectionWithClusterNameConf)
	got, err := (&profilesPipelineTranslator{}).Translate(conf)
	require.NoError(t, err)

	k8sTranslator, ok := got.Processors.Get(component.MustNewIDWithName("k8s_attributes", "profiles"))
	require.True(t, ok)
	cfg, err := k8sTranslator.Translate(conf)
	require.NoError(t, err)
	k8sCfg, ok := cfg.(*k8sattributesprocessor.Config)
	require.True(t, ok)
	require.Len(t, k8sCfg.Association, 2)
	require.Len(t, k8sCfg.Association[0].Sources, 1)
	assert.Equal(t, "resource_attribute", k8sCfg.Association[0].Sources[0].From)
	assert.Equal(t, "container.id", k8sCfg.Association[0].Sources[0].Name)
	require.Len(t, k8sCfg.Association[1].Sources, 1)
	assert.Equal(t, "resource_attribute", k8sCfg.Association[1].Sources[0].From)
	assert.Equal(t, "k8s.pod.ip", k8sCfg.Association[1].Sources[0].Name)
	require.NoError(t, k8sCfg.Validate())

	clusterNameTranslator, ok := got.Processors.Get(component.MustNewIDWithName("transform", "set_cluster_name"))
	require.True(t, ok)
	cfg, err = clusterNameTranslator.Translate(conf)
	require.NoError(t, err)
	clusterNameCfg, ok := cfg.(*transformprocessor.Config)
	require.True(t, ok)
	require.Len(t, clusterNameCfg.ProfileStatements, 1)
	assert.Equal(t, []string{`set(resource.attributes["k8s.cluster.name"], "test-cluster")`}, clusterNameCfg.ProfileStatements[0].Statements)
}

func TestProfilesServiceNameOwnedByResourceProfiles(t *testing.T) {
	testCases := map[string]struct {
		k8sMode string
		ecs     bool
	}{
		"EC2": {},
		"ECS": {ecs: true},
		"EKS": {k8sMode: config.ModeEKS},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlpreceiver.ClearConfigCache)
			resetGlobalConfig(t, "us-east-1")
			setKubernetesMode(t, testCase.k8sMode)
			setECS(t, testCase.ecs)
			t.Setenv("K8S_NODE_NAME", "node_name_from_env")
			conf := confmap.NewFromStringMap(map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"cluster_name":        "test-cluster",
					"resource_attributes": map[string]interface{}{"team": "cloudwatch"},
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{},
					},
				},
			})
			got, err := (&profilesPipelineTranslator{}).Translate(conf)
			require.NoError(t, err)
			keys := got.Processors.Keys()
			require.Equal(t, "resource/profiles", keys[len(keys)-1].String())
			for _, id := range keys[:len(keys)-1] {
				processorTranslator, ok := got.Processors.Get(id)
				require.True(t, ok)
				cfg, err := processorTranslator.Translate(conf)
				require.NoError(t, err)
				switch c := cfg.(type) {
				case *transformprocessor.Config:
					for _, cs := range c.ProfileStatements {
						for _, statement := range cs.Statements {
							if !strings.HasPrefix(statement, `set(resource.attributes["service.name"]`) {
								assert.NotContains(t, statement, `resource.attributes["service.name"]`, id.String())
								continue
							}
							assert.Contains(t, statement, `where resource.attributes["service.name"] == nil`, id.String())
							assert.Regexp(t, `^set\(resource\.attributes\["service\.name"\], resource\.attributes\["[^"]+"\]\) where `, statement, id.String())
						}
					}
				case *resourceprocessor.Config:
					for _, action := range c.AttributesActions {
						assert.NotEqual(t, serviceNameAttribute, action.Key, id.String())
					}
				case *k8sattributesprocessor.Config:
					assert.NotContains(t, c.Extract.Metadata, serviceNameAttribute, id.String())
					for _, annotation := range c.Extract.Annotations {
						assert.NotEqual(t, serviceNameAttribute, annotation.TagName, id.String())
					}
					for _, label := range c.Extract.Labels {
						assert.NotEqual(t, serviceNameAttribute, label.TagName, id.String())
					}
				}
			}
		})
	}
}

func assertProfilesInvariants(t *testing.T, got *common.ComponentTranslators) {
	t.Helper()
	keys := got.Processors.Keys()
	require.NotEmpty(t, keys)
	for _, id := range keys {
		assert.NotEqual(t, "batch", id.Type().String())
	}
	assert.Equal(t, "transform/otlp_scope", keys[0].String())
	assert.Equal(t, "resource/profiles", keys[len(keys)-1].String())
	resourceDetectionIndex, identityIndex := -1, -1
	for i, id := range keys {
		switch id.String() {
		case "resourcedetection/opentelemetry", "resourcedetection/profiles":
			resourceDetectionIndex = i
		case "transform/identity":
			identityIndex = i
		case "resource/opentelemetry":
			assert.Equal(t, 1, i)
		}
	}
	assert.GreaterOrEqual(t, identityIndex, 0)
	assert.Greater(t, identityIndex, resourceDetectionIndex)
}

func containsSubstring(statements []string, substring string) bool {
	for _, statement := range statements {
		if strings.Contains(statement, substring) {
			return true
		}
	}
	return false
}

func setKubernetesMode(t *testing.T, mode string) {
	t.Helper()
	context.CurrentContext().SetKubernetesMode(mode)
	t.Cleanup(func() { context.CurrentContext().SetKubernetesMode("") })
}

func setECS(t *testing.T, ecs bool) {
	t.Helper()
	previous := ecsutil.GetECSUtilSingleton().Region
	if ecs {
		ecsutil.GetECSUtilSingleton().Region = "us-east-1"
	} else {
		ecsutil.GetECSUtilSingleton().Region = ""
	}
	t.Cleanup(func() { ecsutil.GetECSUtilSingleton().Region = previous })
}

func resetGlobalConfig(t *testing.T, region string) {
	t.Helper()
	previous := agent.Global_Config
	t.Cleanup(func() {
		agent.Global_Config = previous
	})
	agent.Global_Config = agent.Agent{Region: region}
	stubEC2ServiceName(t, "")
}

func stubEC2ServiceName(t *testing.T, name string) {
	t.Helper()
	previous := EC2ServiceNameProvider
	t.Cleanup(func() { EC2ServiceNameProvider = previous })
	EC2ServiceNameProvider = func() string { return name }
}

func profilesResourceProcessorConfig(t *testing.T, conf *confmap.Conf) *resourceprocessor.Config {
	t.Helper()
	got, err := (&profilesPipelineTranslator{}).Translate(conf)
	require.NoError(t, err)
	processorTranslator, ok := got.Processors.Get(component.MustNewIDWithName("resource", "profiles"))
	require.True(t, ok)
	cfg, err := processorTranslator.Translate(conf)
	require.NoError(t, err)
	processorCfg, ok := cfg.(*resourceprocessor.Config)
	require.True(t, ok)
	return processorCfg
}
