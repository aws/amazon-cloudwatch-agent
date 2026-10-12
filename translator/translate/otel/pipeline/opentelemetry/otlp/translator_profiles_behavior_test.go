// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapstructure"
	"github.com/aws/amazon-cloudwatch-agent/service/configprovider"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toyamlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

type profilesBehaviorCase struct {
	mode           string
	k8sMode        string
	ecs            bool
	ec2ServiceName string
	attributes     map[string]string
	want           map[string]string
	wantPresent    []string
	wantAbsent     []string
}

func TestProfilesServiceNameBehavior(t *testing.T) {
	testCases := map[string]profilesBehaviorCase{
		"EC2/Absent":             {want: map[string]string{serviceNameAttribute: "unknown_service"}},
		"EC2/SDKUnknownService":  {attributes: map[string]string{serviceNameAttribute: "unknown_service:java"}, want: map[string]string{serviceNameAttribute: "unknown_service:java"}},
		"EC2/BareUnknownService": {attributes: map[string]string{serviceNameAttribute: "unknown_service"}, want: map[string]string{serviceNameAttribute: "unknown_service"}},
		"EC2/SenderStamped":      {attributes: map[string]string{serviceNameAttribute: "payments"}, want: map[string]string{serviceNameAttribute: "payments"}},
		"EC2/PrefixIsNotUnknown": {attributes: map[string]string{serviceNameAttribute: "my-unknown_service:java"}, want: map[string]string{serviceNameAttribute: "my-unknown_service:java"}},
		"EKS/Absent":             {k8sMode: config.ModeEKS, want: map[string]string{serviceNameAttribute: "unknown_service"}},
		"EKS/SDKUnknownService":  {k8sMode: config.ModeEKS, attributes: map[string]string{serviceNameAttribute: "unknown_service:dotnet"}, want: map[string]string{serviceNameAttribute: "unknown_service:dotnet"}},
		"EKS/SenderStamped":      {k8sMode: config.ModeEKS, attributes: map[string]string{serviceNameAttribute: "checkout"}, want: map[string]string{serviceNameAttribute: "checkout"}},
		"EKS/PodMetadataCleanedUp": {
			k8sMode: config.ModeEKS,
			attributes: map[string]string{
				serviceNameAttribute:                            "checkout",
				"app.kubernetes.io/name":                        "payments",
				"app.kubernetes.io/instance":                    "payments-1",
				"app.kubernetes.io/version":                     "1.2.3",
				"resource.opentelemetry.io/service.name":        "payments",
				"resource.opentelemetry.io/service.namespace":   "shop",
				"resource.opentelemetry.io/service.instance.id": "shop/payments-1/app",
				"resource.opentelemetry.io/service.version":     "1.2.3",
				"k8s.namespace.name":                            "shop",
			},
			want:        map[string]string{serviceNameAttribute: "checkout"},
			wantPresent: []string{"k8s.namespace.name"},
			wantAbsent: []string{
				"app.kubernetes.io/name",
				"app.kubernetes.io/instance",
				"app.kubernetes.io/version",
				"resource.opentelemetry.io/service.name",
				"resource.opentelemetry.io/service.namespace",
				"resource.opentelemetry.io/service.instance.id",
				"resource.opentelemetry.io/service.version",
			},
		},
		"EKS/AnnotationNamesService": {
			k8sMode: config.ModeEKS,
			attributes: map[string]string{
				"resource.opentelemetry.io/service.name":      "payments",
				"resource.opentelemetry.io/service.namespace": "shop-ns",
				"app.kubernetes.io/name":                      "payments-label",
				"k8s.deployment.name":                         "payments-deploy",
				"k8s.namespace.name":                          "shop",
			},
			want:       map[string]string{serviceNameAttribute: "payments", "service.namespace": "shop-ns"},
			wantAbsent: []string{"resource.opentelemetry.io/service.name", "resource.opentelemetry.io/service.namespace", "app.kubernetes.io/name"},
		},
		"EKS/InstanceLabelBeforeNameLabel": {
			k8sMode:    config.ModeEKS,
			attributes: map[string]string{"app.kubernetes.io/instance": "payments-1", "app.kubernetes.io/name": "payments", "k8s.namespace.name": "shop"},
			want:       map[string]string{serviceNameAttribute: "payments-1", "service.namespace": "shop"},
			wantAbsent: []string{"app.kubernetes.io/instance", "app.kubernetes.io/name"},
		},
		"EKS/NameLabelBeforeWorkload": {
			k8sMode:    config.ModeEKS,
			attributes: map[string]string{"app.kubernetes.io/name": "payments", "k8s.deployment.name": "payments-deploy"},
			want:       map[string]string{serviceNameAttribute: "payments"},
		},
		"EKS/WorkloadBeforePod": {
			k8sMode:    config.ModeEKS,
			attributes: map[string]string{"k8s.statefulset.name": "payments-db", "k8s.pod.name": "payments-db-0"},
			want:       map[string]string{serviceNameAttribute: "payments-db"},
		},
		"EKS/PodBeforeContainer": {
			k8sMode:    config.ModeEKS,
			attributes: map[string]string{"k8s.pod.name": "payments-db-0", "k8s.container.name": "app"},
			want:       map[string]string{serviceNameAttribute: "payments-db-0"},
		},
		"EKS/SenderBeatsPodMetadata": {
			k8sMode:    config.ModeEKS,
			attributes: map[string]string{serviceNameAttribute: "checkout", "service.namespace": "front", "resource.opentelemetry.io/service.name": "payments", "k8s.namespace.name": "shop"},
			want:       map[string]string{serviceNameAttribute: "checkout", "service.namespace": "front"},
		},
		"EKS/SDKUnknownServiceNotReInferred": {
			k8sMode:    config.ModeEKS,
			attributes: map[string]string{serviceNameAttribute: "unknown_service:java", "resource.opentelemetry.io/service.name": "payments", "app.kubernetes.io/name": "payments"},
			want:       map[string]string{serviceNameAttribute: "unknown_service:java"},
			wantAbsent: []string{"resource.opentelemetry.io/service.name", "app.kubernetes.io/name"},
		},
		"ECS/TaskFamilyNamesService": {
			ecs:        true,
			attributes: map[string]string{"aws.ecs.task.family": "payments-task"},
			want:       map[string]string{serviceNameAttribute: "payments-task"},
		},
		"ECS/SenderBeatsTaskFamily": {
			ecs:        true,
			attributes: map[string]string{serviceNameAttribute: "payments", "aws.ecs.task.family": "payments-task"},
			want:       map[string]string{serviceNameAttribute: "payments"},
		},
		"ECS/SDKUnknownServiceBeatsTaskFamily": {
			ecs:        true,
			attributes: map[string]string{serviceNameAttribute: "unknown_service:java", "aws.ecs.task.family": "payments-task"},
			want:       map[string]string{serviceNameAttribute: "unknown_service:java"},
		},
		"ECS/Absent": {
			ecs:  true,
			want: map[string]string{serviceNameAttribute: "unknown_service"},
		},
		"EC2/InferredFromHost": {
			ec2ServiceName: "web-tier",
			want:           map[string]string{serviceNameAttribute: "web-tier"},
		},
		"EC2/SenderBeatsInferred": {
			ec2ServiceName: "web-tier",
			attributes:     map[string]string{serviceNameAttribute: "payments"},
			want:           map[string]string{serviceNameAttribute: "payments"},
		},
		"EC2/SDKUnknownServiceBeatsInferred": {
			ec2ServiceName: "web-tier",
			attributes:     map[string]string{serviceNameAttribute: "unknown_service:java"},
			want:           map[string]string{serviceNameAttribute: "unknown_service:java"},
		},
	}
	runProfilesBehaviorCases(t, testCases)
}

func TestProfilesEnrichmentBehavior(t *testing.T) {
	testCases := map[string]profilesBehaviorCase{
		"EC2/ResourceIDAndEnvironmentFromASG": {
			attributes: map[string]string{
				"cloud.platform":                    "aws_ec2",
				"cloud.region":                      "us-east-1",
				"cloud.account.id":                  "123456789012",
				"host.id":                           "i-0123456789abcdef0",
				"ec2.tag.aws:autoscaling:groupName": "web-asg",
			},
			want: map[string]string{
				"cloud.resource_id":           "arn:aws:ec2:us-east-1:123456789012:instance/i-0123456789abcdef0",
				"deployment.environment.name": "aws_ec2:web-asg",
			},
		},
		"EC2/EnvironmentDefaultWithoutASG": {
			attributes: map[string]string{
				"cloud.platform":   "aws_ec2",
				"cloud.region":     "us-east-1",
				"cloud.account.id": "123456789012",
				"host.id":          "i-0123456789abcdef0",
			},
			want: map[string]string{
				"cloud.resource_id":           "arn:aws:ec2:us-east-1:123456789012:instance/i-0123456789abcdef0",
				"deployment.environment.name": "aws_ec2:default",
			},
		},
		"EC2/SenderValuesPreserved": {
			attributes: map[string]string{
				"cloud.platform":              "aws_ec2",
				"cloud.region":                "us-east-1",
				"cloud.account.id":            "123456789012",
				"host.id":                     "i-0123456789abcdef0",
				"cloud.resource_id":           "arn:aws:ec2:us-east-1:123456789012:instance/i-custom",
				"deployment.environment.name": "prod",
			},
			want: map[string]string{
				"cloud.resource_id":           "arn:aws:ec2:us-east-1:123456789012:instance/i-custom",
				"deployment.environment.name": "prod",
			},
		},
		"EC2/NothingDetected": {
			wantAbsent: []string{"cloud.resource_id", "deployment.environment.name"},
		},
		"ECS/ResourceIDAndEnvironmentFromCluster": {
			ecs: true,
			attributes: map[string]string{
				"cloud.platform":      "aws_ecs",
				"aws.ecs.task.arn":    "arn:aws:ecs:us-east-1:123456789012:task/shop/0123456789abcdef0",
				"aws.ecs.cluster.arn": "arn:aws:ecs:us-east-1:123456789012:cluster/shop",
			},
			want: map[string]string{
				"cloud.resource_id":           "arn:aws:ecs:us-east-1:123456789012:task/shop/0123456789abcdef0",
				"aws.ecs.cluster.name":        "shop",
				"deployment.environment.name": "aws_ecs:shop",
			},
		},
		"EKS/ResourceIDAndEnvironment": {
			k8sMode: config.ModeEKS,
			attributes: map[string]string{
				"cloud.platform":     "aws_eks",
				"cloud.region":       "us-east-1",
				"cloud.account.id":   "123456789012",
				"k8s.cluster.name":   "test-cluster",
				"k8s.namespace.name": "shop",
			},
			want: map[string]string{
				"cloud.resource_id":           "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
				"deployment.environment.name": "aws_eks:test-cluster/shop",
			},
		},
		"AKS/ResourceIDFromNodeResourceGroup": {
			mode:    config.ModeAzureVM,
			k8sMode: config.ModeAKS,
			attributes: map[string]string{
				"cloud.platform":           "azure.aks",
				"cloud.account.id":         "00000000-0000-0000-0000-000000000000",
				"azure.resourcegroup.name": "MC_shop-rg_test-cluster_westeurope",
				"k8s.cluster.name":         "test-cluster",
				"k8s.namespace.name":       "shop",
			},
			want: map[string]string{
				"cloud.resource_id":           "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/shop-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster",
				"deployment.environment.name": "azure.aks:test-cluster/shop",
			},
			wantAbsent: []string{"_tmp.azure.resourcegroup.name"},
		},
		"AzureVM/ResourceIDAndEnvironment": {
			mode: config.ModeAzureVM,
			attributes: map[string]string{
				"cloud.platform":           "azure.vm",
				"cloud.account.id":         "00000000-0000-0000-0000-000000000000",
				"azure.resourcegroup.name": "shop-rg",
				"azure.vm.name":            "web-1",
			},
			want: map[string]string{
				"cloud.resource_id":           "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/shop-rg/providers/Microsoft.Compute/virtualMachines/web-1",
				"deployment.environment.name": "azure.vm:shop-rg",
			},
		},
		"GCE/ResourceIDEnvironmentAndHostType": {
			mode: config.ModeGCE,
			attributes: map[string]string{
				"cloud.platform":          "gcp_compute_engine",
				"cloud.account.id":        "my-project",
				"cloud.availability_zone": "europe-west1-b",
				"host.name":               "web-1",
				"host.type":               "projects/123456789012/machineTypes/n2-standard-4",
			},
			want: map[string]string{
				"cloud.resource_id":           "//compute.googleapis.com/projects/my-project/zones/europe-west1-b/instances/web-1",
				"deployment.environment.name": "gcp_compute_engine:my-project",
				"host.type":                   "n2-standard-4",
			},
		},
	}
	runProfilesBehaviorCases(t, testCases)
}

func runProfilesBehaviorCases(t *testing.T, testCases map[string]profilesBehaviorCase) {
	t.Helper()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(otlpreceiver.ClearConfigCache)
			resetGlobalConfig(t, "us-east-1")
			stubEC2ServiceName(t, testCase.ec2ServiceName)
			setMode(t, testCase.mode)
			setKubernetesMode(t, testCase.k8sMode)
			setECS(t, testCase.ecs)
			conf := confmap.NewFromStringMap(otlpSectionWithClusterNameConf)
			got, err := (&profilesPipelineTranslator{}).Translate(conf)
			require.NoError(t, err)

			profiles := pprofile.NewProfiles()
			resource := profiles.ResourceProfiles().AppendEmpty().Resource()
			for key, value := range testCase.attributes {
				resource.Attributes().PutStr(key, value)
			}

			loaded := loadProcessorsAsAgent(t, got, conf)
			sink := new(consumertest.ProfilesSink)
			resourceProfiles := newProfilesProcessor(t, resourceprocessor.NewFactory(), loaded, "resource", profilesExporterName, sink)
			identity := newProfilesProcessor(t, transformprocessor.NewFactory(), loaded, "transform", "identity", resourceProfiles)
			require.NoError(t, identity.ConsumeProfiles(context.Background(), profiles))

			require.Len(t, sink.AllProfiles(), 1)
			attributes := sink.AllProfiles()[0].ResourceProfiles().At(0).Resource().Attributes()
			for key, want := range testCase.want {
				value, ok := attributes.Get(key)
				require.True(t, ok, key)
				assert.Equal(t, want, value.Str(), key)
			}
			for _, key := range testCase.wantPresent {
				_, ok := attributes.Get(key)
				assert.True(t, ok, key)
			}
			for _, key := range testCase.wantAbsent {
				_, ok := attributes.Get(key)
				assert.False(t, ok, key)
			}
			value, ok := attributes.Get(serviceNameAttribute)
			require.True(t, ok)
			assert.NotEmpty(t, value.Str())
		})
	}
}

func loadProcessorsAsAgent(t *testing.T, translators *common.ComponentTranslators, conf *confmap.Conf) *confmap.Conf {
	t.Helper()
	processors := map[string]any{}
	translators.Processors.Range(func(translator common.ComponentTranslator) {
		cfg, err := translator.Translate(conf)
		require.NoError(t, err)
		processors[translator.ID().String()] = cfg
	})
	marshalled, err := mapstructure.Marshal(map[string]any{"processors": processors})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(toyamlconfig.ToYamlConfig(marshalled)), 0600))

	resolver, err := confmap.NewResolver(configprovider.GetSettings([]string{"file:" + path}, zap.NewNop()).ResolverSettings)
	require.NoError(t, err)
	loaded, err := resolver.Resolve(context.Background())
	require.NoError(t, err)
	return loaded
}

func newProfilesProcessor(t *testing.T, base processor.Factory, loaded *confmap.Conf, typ, name string, next xconsumer.Profiles) xprocessor.Profiles {
	t.Helper()
	factory, ok := base.(xprocessor.Factory)
	require.True(t, ok)
	cfg := factory.CreateDefaultConfig()
	sub, err := loaded.Sub("processors::" + component.MustNewIDWithName(typ, name).String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	proc, err := factory.CreateProfiles(context.Background(), processortest.NewNopSettings(factory.Type()), cfg, next)
	require.NoError(t, err)
	require.NoError(t, proc.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, proc.Shutdown(context.Background())) })
	return proc
}

func setMode(t *testing.T, mode string) {
	t.Helper()
	previous := translatorcontext.CurrentContext().Mode()
	if mode != "" {
		translatorcontext.CurrentContext().SetMode(mode)
	}
	t.Cleanup(func() { translatorcontext.CurrentContext().SetMode(previous) })
}
