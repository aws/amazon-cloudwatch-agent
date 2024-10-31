// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

type mockEntityStore struct {
	entries                    []entityStoreEntry
	podToServiceEnvironmentMap map[string]entitystore.ServiceEnvironment
}

type entityStoreEntry struct {
	logGroupName    entitystore.LogGroupName
	serviceName     string
	environmentName string
}

func newMockEntityStore() *mockEntityStore {
	return &mockEntityStore{
		entries:                    make([]entityStoreEntry, 0),
		podToServiceEnvironmentMap: make(map[string]entitystore.ServiceEnvironment),
	}
}

func newMockAddPodServiceEnvironmentMapping(es *mockEntityStore) func(string, string, string, string) {
	return func(podName string, serviceName string, deploymentName string, serviceNameSource string) {
		es.podToServiceEnvironmentMap[podName] = entitystore.ServiceEnvironment{ServiceName: serviceName, Environment: deploymentName, ServiceNameSource: serviceNameSource}
	}
}

func newAddToMockEntityStore(rs *mockEntityStore) func(entitystore.LogGroupName, string, string) {
	return func(logGroupName entitystore.LogGroupName, serviceName string, environmentName string) {
		rs.entries = append(rs.entries, entityStoreEntry{
			logGroupName:    logGroupName,
			serviceName:     serviceName,
			environmentName: environmentName,
		})
	}
}

func newMockGetMetricAttributesFromEntityStore() func() map[string]*string {
	mockPlatform := "AWS::EC2"
	mockInstanceID := "i-123456789"
	mockAutoScalingGroup := "auto-scaling"
	return func() map[string]*string {
		return map[string]*string{
			entitystore.PlatformType:  &mockPlatform,
			entitystore.InstanceIDKey: &mockInstanceID,
			entitystore.ASGKey:        &mockAutoScalingGroup,
		}
	}
}

func newMockGetServiceNameAndSource(service, source string) func() (string, string) {
	return func() (string, string) {
		return service, source
	}
}

func newMockGetEC2InfoFromEntityStore(instance, accountId, asg string) func() entitystore.EC2Info {
	return func() entitystore.EC2Info {
		return entitystore.EC2Info{
			InstanceID:       instance,
			AccountID:        accountId,
			AutoScalingGroup: asg,
		}
	}
}

// This helper function creates a test logger
// so that it can send the log messages into a
// temporary buffer for pattern matching
func CreateTestLogger(buf *bytes.Buffer) *zap.Logger {
	writer := zapcore.AddSync(buf)

	// Create a custom zapcore.Core that writes to the buffer
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)
	logger := zap.New(core)
	return logger
}

func TestProcessMetricsLogGroupAssociation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := newAwsEntityProcessor(&Config{
		EntityType: attributeService,
	}, logger)
	ctx := context.Background()

	// empty metrics, no action
	// metrics with no log group names, no action
	// metrics with no service/environment, no action
	// metrics with log group name and service, add to rs
	// metrics with log group name and env, add to rs
	// metrics with log group name and both, add to rs
	// metrics with two log group names, add both
	// metrics with two resourcemetrics, add both
	tests := []struct {
		name    string
		metrics pmetric.Metrics
		want    []entityStoreEntry
	}{
		{
			name:    "EmptyMetrics",
			metrics: pmetric.NewMetrics(),
			want:    []entityStoreEntry{},
		},
		{
			name:    "NoLogGroupNames",
			metrics: generateMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment"),
			want:    []entityStoreEntry{},
		},
		{
			name:    "NoServiceOrEnvironment",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group"),
			want:    []entityStoreEntry{},
		},
		{
			name:    "LogGroupNameAndService",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group", attributeServiceName, "test-service"),
			want:    []entityStoreEntry{{logGroupName: "test-log-group", serviceName: "test-service"}},
		},
		{
			name:    "LogGroupNameAndEnvironment",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group", attributeDeploymentEnvironment, "test-environment"),
			want:    []entityStoreEntry{{logGroupName: "test-log-group", environmentName: "test-environment"}},
		},
		{
			name:    "LogGroupNameAndServiceAndEnvironment",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group", attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment"),
			want:    []entityStoreEntry{{logGroupName: "test-log-group", serviceName: "test-service", environmentName: "test-environment"}},
		},
		{
			name:    "TwoLogGroupNames",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group1&test-log-group2", attributeServiceName, "test-service"),
			want: []entityStoreEntry{
				{logGroupName: "test-log-group1", serviceName: "test-service"},
				{logGroupName: "test-log-group2", serviceName: "test-service"},
			},
		},
		{
			name:    "EmptyLogGroupNames",
			metrics: generateMetrics(attributeAwsLogGroupNames, "&&test-log-group1&&test-log-group2&&", attributeServiceName, "test-service"),
			want: []entityStoreEntry{
				{logGroupName: "test-log-group1", serviceName: "test-service"},
				{logGroupName: "test-log-group2", serviceName: "test-service"},
			},
		},
		{
			name:    "TwoResourceMetrics",
			metrics: generateMetricsWithTwoResources(),
			want: []entityStoreEntry{
				{logGroupName: "test-log-group1", serviceName: "test-service1"},
				{logGroupName: "test-log-group2", serviceName: "test-service2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newMockEntityStore()
			addToEntityStore = newAddToMockEntityStore(rs)
			_, err := p.processMetrics(ctx, tt.metrics)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, rs.entries)
		})
	}
}

func TestProcessMetricsForAddingPodToServiceMap(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := newAwsEntityProcessor(&Config{ClusterName: "test-cluster", EntityType: attributeService}, logger)
	ctx := context.Background()
	tests := []struct {
		name    string
		metrics pmetric.Metrics
		k8sMode string
		want    map[string]entitystore.ServiceEnvironment
	}{
		{
			name:    "WithPodNameAndServiceNameNoSource",
			metrics: generateMetrics(attributeServiceName, "test-service", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "test-service", ServiceNameSource: entitystore.ServiceNameSourceUnknown}},
			k8sMode: config.ModeEKS,
		},
		{
			name:    "WithPodNameAndServiceNameHasSource",
			metrics: generateMetrics(attributeServiceName, "test-service", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf", entityattributes.AttributeEntityServiceNameSource, "Instrumentation"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "test-service", ServiceNameSource: entitystore.ServiceNameSourceInstrumentation}},
			k8sMode: config.ModeEKS,
		},
		{
			name:    "WithPodNameAndServiceNameHasSourceDefaultEnvironmentEKS",
			metrics: generateMetrics(attributeServiceName, "test-service", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf", semconv.AttributeK8SNamespaceName, "test-namespace", entityattributes.AttributeEntityServiceNameSource, "Instrumentation"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "test-service", Environment: "eks:test-cluster/test-namespace", ServiceNameSource: entitystore.ServiceNameSourceInstrumentation}},
			k8sMode: config.ModeEKS,
		},
		{
			name:    "WithPodNameAndServiceNameHasSourceDefaultEnvironmentK8SEC2",
			metrics: generateMetrics(attributeServiceName, "test-service", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf", semconv.AttributeK8SNamespaceName, "test-namespace", entityattributes.AttributeEntityServiceNameSource, "Instrumentation"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "test-service", Environment: "k8s:test-cluster/test-namespace", ServiceNameSource: entitystore.ServiceNameSourceInstrumentation}},
			k8sMode: config.ModeK8sEC2,
		},
		{
			name:    "WithPodNameAndServiceNameHasSourceDefaultEnvironmentK8SOnPrem",
			metrics: generateMetrics(attributeServiceName, "test-service", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf", semconv.AttributeK8SNamespaceName, "test-namespace", entityattributes.AttributeEntityServiceNameSource, "Instrumentation"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "test-service", Environment: "k8s:test-cluster/test-namespace", ServiceNameSource: entitystore.ServiceNameSourceInstrumentation}},
			k8sMode: config.ModeK8sOnPrem,
		},
		{
			name:    "WithPodNameAndServiceEnvironmentNameNoSource",
			metrics: generateMetrics(attributeServiceName, "test-service", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf", attributeDeploymentEnvironment, "test-deployment"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "test-service", Environment: "test-deployment", ServiceNameSource: entitystore.ServiceNameSourceUnknown}},
			k8sMode: config.ModeK8sEC2,
		},
		{
			name:    "WithPodNameAndServiceEnvironmentNameHasSource",
			metrics: generateMetrics(attributeServiceName, "test-service", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf", attributeDeploymentEnvironment, "test-deployment", entityattributes.AttributeEntityServiceNameSource, "Instrumentation"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "test-service", Environment: "test-deployment", ServiceNameSource: entitystore.ServiceNameSourceInstrumentation}},
			k8sMode: config.ModeK8sEC2,
		},
		{
			name:    "WithPodNameAndAttributeService",
			metrics: generateMetrics(attributeService, "test-service", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf", entityattributes.AttributeEntityServiceNameSource, "Instrumentation"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "test-service", ServiceNameSource: entitystore.ServiceNameSourceInstrumentation}},
			k8sMode: config.ModeK8sOnPrem,
		},
		{
			name:    "WithPodNameAndWorkload",
			metrics: generateMetrics(attributeServiceName, "cloudwatch-agent-adhgaf", semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf", entityattributes.AttributeEntityServiceNameSource, "K8sWorkload"),
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "cloudwatch-agent-adhgaf", ServiceNameSource: entitystore.ServiceNameSourceK8sWorkload}},
			k8sMode: config.ModeEKS,
		},
		{
			name:    "WithPodNameAndEmptyServiceAndEnvironmentName",
			metrics: generateMetrics(semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf"),
			k8sMode: config.ModeEKS,
			want:    map[string]entitystore.ServiceEnvironment{"cloudwatch-agent-adhgaf": {ServiceName: "cloudwatch-agent-adhgaf", ServiceNameSource: entitystore.ServiceNameSourceK8sWorkload}},
		},
		{
			name:    "WithEmptyPodName",
			metrics: generateMetrics(),
			k8sMode: config.ModeEKS,
			want:    map[string]entitystore.ServiceEnvironment{},
		},
		{
			name:    "WithEmptyKubernetesMode",
			metrics: generateMetrics(semconv.AttributeK8SPodName, "cloudwatch-agent-adhgaf"),
			want:    map[string]entitystore.ServiceEnvironment{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := newMockEntityStore()
			addPodToServiceEnvironmentMap = newMockAddPodServiceEnvironmentMapping(es)
			p.config.KubernetesMode = tt.k8sMode
			_, err := p.processMetrics(ctx, tt.metrics)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, es.podToServiceEnvironmentMap)
		})
	}
}

func TestProcessMetricsResourceAttributeScraping(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()
	tests := []struct {
		name                          string
		platform                      string
		kubernetesMode                string
		clusterName                   string
		metrics                       pmetric.Metrics
		mockServiceNameSource         func() (string, string)
		mockGetEC2InfoFromEntityStore func() entitystore.EC2Info
		want                          map[string]any
	}{
		{
			name:     "EmptyMetrics",
			platform: config.ModeEC2,
			metrics:  pmetric.NewMetrics(),
			want:     map[string]any{},
		},
		//NOTE 2 SELF: These tests assume that we are on the EC2 platform, so make sure to mock the ServiceNameSource function
		{
			name:                          "ResourceAttributeServiceNameOnly",
			platform:                      config.ModeEC2,
			metrics:                       generateMetrics(attributeServiceName, "test-service"),
			mockServiceNameSource:         newMockGetServiceNameAndSource("test-service-name", "Instrumentation"),
			mockGetEC2InfoFromEntityStore: newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", ""),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "Unknown",
				entityattributes.AttributeEntityDeploymentEnvironment: "ec2:default",
				attributeServiceName:                                  "test-service",
			},
		},
		{
			name:                          "ResourceAttributeEnvironmentOnly",
			platform:                      config.ModeEC2,
			metrics:                       generateMetrics(attributeDeploymentEnvironment, "test-environment"),
			mockServiceNameSource:         newMockGetServiceNameAndSource("unknown_service", "Unknown"),
			mockGetEC2InfoFromEntityStore: newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", ""),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "unknown_service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "Unknown",

				attributeDeploymentEnvironment: "test-environment",
			},
		},
		{
			name:                          "ResourceAttributeServiceNameAndEnvironment",
			platform:                      config.ModeEC2,
			metrics:                       generateMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment"),
			mockServiceNameSource:         newMockGetServiceNameAndSource("test-service-name", "Instrumentation"),
			mockGetEC2InfoFromEntityStore: newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", "test-auto-scaling"),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				attributeServiceName:                                  "test-service",
				attributeDeploymentEnvironment:                        "test-environment",
				entityattributes.AttributeEntityServiceNameSource:     "Unknown",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityAutoScalingGroup:      "test-auto-scaling",
			},
		},
		{
			name:           "ResourceAttributeWorkloadFallback",
			kubernetesMode: config.ModeEKS,
			clusterName:    "test-cluster",
			metrics:        generateMetrics(semconv.AttributeK8SNamespaceName, "test-namespace", semconv.AttributeK8SDeploymentName, "test-workload", semconv.AttributeK8SNodeName, "test-node"),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-workload",
				entityattributes.AttributeEntityDeploymentEnvironment: "eks:test-cluster/test-namespace",
				entityattributes.AttributeEntityCluster:               "test-cluster",
				entityattributes.AttributeEntityNamespace:             "test-namespace",
				entityattributes.AttributeEntityNode:                  "test-node",
				entityattributes.AttributeEntityWorkload:              "test-workload",
				entityattributes.AttributeEntityServiceNameSource:     "K8sWorkload",
				entityattributes.AttributeEntityPlatformType:          "AWS::EKS",
				semconv.AttributeK8SNamespaceName:                     "test-namespace",
				semconv.AttributeK8SDeploymentName:                    "test-workload",
				semconv.AttributeK8SNodeName:                          "test-node",
			},
		},
		{
			name:                          "ResourceAttributeEnvironmentFallbackToASG",
			platform:                      config.ModeEC2,
			metrics:                       generateMetrics(),
			mockServiceNameSource:         newMockGetServiceNameAndSource("unknown_service", "Unknown"),
			mockGetEC2InfoFromEntityStore: newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", "test-asg"),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "unknown_service",
				entityattributes.AttributeEntityDeploymentEnvironment: "ec2:test-asg",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "Unknown",
				entityattributes.AttributeEntityAutoScalingGroup:      "test-asg",
			},
		},
		{
			name:                          "ResourceAttributeEnvironmentFallbackToDefault",
			platform:                      config.ModeEC2,
			metrics:                       generateMetrics(),
			mockServiceNameSource:         newMockGetServiceNameAndSource("unknown_service", "Unknown"),
			mockGetEC2InfoFromEntityStore: newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", ""),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "unknown_service",
				entityattributes.AttributeEntityDeploymentEnvironment: "ec2:default",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "Unknown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make copy of original functions to use as resets later to prevent failing test when tests are ran in bulk
			resetServiceNameSource := getServiceNameSource
			if tt.mockServiceNameSource != nil {
				getServiceNameSource = tt.mockServiceNameSource
			}
			if tt.mockGetEC2InfoFromEntityStore != nil {
				getEC2InfoFromEntityStore = tt.mockGetEC2InfoFromEntityStore
			}
			p := newAwsEntityProcessor(&Config{EntityType: attributeService, ClusterName: tt.clusterName}, logger)
			p.config.Platform = tt.platform
			p.config.KubernetesMode = tt.kubernetesMode
			_, err := p.processMetrics(ctx, tt.metrics)
			assert.NoError(t, err)
			rm := tt.metrics.ResourceMetrics()
			if rm.Len() > 0 {
				assert.Equal(t, tt.want, rm.At(0).Resource().Attributes().AsRaw())
			}
			getServiceNameSource = resetServiceNameSource
		})
	}
}

func TestProcessMetricsResourceEntityProcessing(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()
	tests := []struct {
		name      string
		metrics   pmetric.Metrics
		want      map[string]any
		instance  string
		accountId string
		asg       string
	}{
		{
			name:    "EmptyMetrics",
			metrics: pmetric.NewMetrics(),
			want:    map[string]any{},
		},
		{
			name:      "ResourceEntityEC2",
			metrics:   generateMetrics(),
			instance:  "i-123456789",
			accountId: "0123456789012",
			want: map[string]any{
				"com.amazonaws.cloudwatch.entity.internal.type":           "AWS::Resource",
				"com.amazonaws.cloudwatch.entity.internal.resource.type":  "AWS::EC2::Instance",
				"com.amazonaws.cloudwatch.entity.internal.identifier":     "i-123456789",
				"com.amazonaws.cloudwatch.entity.internal.aws.account.id": "0123456789012",
			},
		},
		{
			name:      "ResourceEntityEC2NoInstance",
			metrics:   generateMetrics(),
			instance:  "",
			accountId: "",
			want:      map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getEC2InfoFromEntityStore = newMockGetEC2InfoFromEntityStore(tt.instance, tt.accountId, tt.asg)
			p := newAwsEntityProcessor(&Config{EntityType: entityattributes.Resource}, logger)
			p.config.Platform = config.ModeEC2
			_, err := p.processMetrics(ctx, tt.metrics)
			assert.NoError(t, err)
			rm := tt.metrics.ResourceMetrics()
			if rm.Len() > 0 {
				assert.Equal(t, tt.want, rm.At(0).Resource().Attributes().AsRaw())
			}
		})
	}
}

func TestAWSEntityProcessorNoSensitiveInfoInLogs(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	logger := CreateTestLogger(&buf)

	configs := []struct {
		name   string
		config *Config
	}{
		{
			name: "EC2Service",
			config: &Config{
				EntityType: entityattributes.Service,
				Platform:   config.ModeEC2,
			},
		},
		{
			name: "EKSService",
			config: &Config{
				EntityType:     entityattributes.Service,
				Platform:       config.ModeEC2,
				KubernetesMode: config.ModeEKS,
				ClusterName:    "test-cluster",
			},
		},
		{
			name: "EC2Resource",
			config: &Config{
				EntityType: entityattributes.Resource,
				Platform:   config.ModeEC2,
			},
		},
		{
			name: "K8sOnPremService",
			config: &Config{
				EntityType:     entityattributes.Service,
				Platform:       config.ModeOnPrem,
				KubernetesMode: config.ModeK8sOnPrem,
				ClusterName:    "test-cluster",
			},
		},
	}

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			buf.Reset()
			processor := newAwsEntityProcessor(cfg.config, logger)

			resetServiceNameSource := getServiceNameSource
			getServiceNameSource = newMockGetServiceNameAndSource("test-service", "UserConfiguration")
			defer func() { getServiceNameSource = resetServiceNameSource }()

			resetGetEC2InfoFromEntityStore := getEC2InfoFromEntityStore
			asgName := "test-asg"
			getEC2InfoFromEntityStore = newMockGetEC2InfoFromEntityStore("i-1234567890abcdef0", "123456789012", asgName)
			defer func() { getEC2InfoFromEntityStore = resetGetEC2InfoFromEntityStore }()

			md := generateTestMetrics()
			_, err := processor.processMetrics(context.Background(), md)
			assert.NoError(t, err)

			logOutput := buf.String()
			assertNoSensitiveInfo(t, logOutput, md, asgName)
		})
	}
}

func generateTestMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	attrs := rm.Resource().Attributes()
	attrs.PutStr(attributeAwsLogGroupNames, "test-log-group")
	attrs.PutStr(attributeServiceName, "test-service")
	attrs.PutStr(attributeDeploymentEnvironment, "test-environment")
	attrs.PutStr(semconv.AttributeK8SPodName, "test-pod")
	attrs.PutStr(semconv.AttributeK8SNamespaceName, "test-namespace")
	attrs.PutStr(semconv.AttributeK8SDeploymentName, "test-deployment")
	attrs.PutStr(semconv.AttributeK8SNodeName, "test-node")

	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("test-metric")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr(attributeServiceName, "datapoint-service-name")
	dp.Attributes().PutStr(attributeDeploymentEnvironment, "datapoint-environment")

	return md
}

func assertNoSensitiveInfo(t *testing.T, logOutput string, md pmetric.Metrics, asgName string) {
	rm := md.ResourceMetrics().At(0)
	attrs := rm.Resource().Attributes()
	dp := rm.ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)

	getStringOrEmpty := func(val pcommon.Value, exists bool) string {
		if !exists {
			return ""
		}
		return val.AsString()
	}

	sensitivePatterns := []string{
		`i-[0-9a-f]{17}`, // EC2 Instance ID regex pattern
		`\d{12}`,         // AWS Account ID regex pattern
		asgName,          // Auto Scaling Group name
		getStringOrEmpty(attrs.Get(attributeAwsLogGroupNames)),
		getStringOrEmpty(attrs.Get(attributeServiceName)),
		getStringOrEmpty(attrs.Get(attributeDeploymentEnvironment)),
		getStringOrEmpty(attrs.Get(semconv.AttributeK8SPodName)),
		getStringOrEmpty(attrs.Get(semconv.AttributeK8SNamespaceName)),
		getStringOrEmpty(attrs.Get(semconv.AttributeK8SDeploymentName)),
		getStringOrEmpty(attrs.Get(semconv.AttributeK8SNodeName)),
		getStringOrEmpty(dp.Attributes().Get(attributeServiceName)),
		getStringOrEmpty(dp.Attributes().Get(attributeDeploymentEnvironment)),
	}

	for _, pattern := range sensitivePatterns {
		assert.NotRegexp(t, pattern, logOutput)
	}
}

func TestProcessMetricsDatapointAttributeScraping(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()
	tests := []struct {
		name                           string
		checkDatapointAttributeRemoval bool
		metrics                        pmetric.Metrics
		mockServiceNameAndSource       func() (string, string)
		mockGetEC2InfoFromEntityStore  func() entitystore.EC2Info
		want                           map[string]any
		wantDatapointAttributes        map[string]any
	}{
		{
			name:    "EmptyMetrics",
			metrics: pmetric.NewMetrics(),
			want:    map[string]any{},
		},
		{
			name:                          "DatapointAttributeServiceNameOnly",
			metrics:                       generateDatapointMetrics(attributeServiceName, "test-service"),
			mockGetEC2InfoFromEntityStore: newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", "auto-scaling"),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityServiceNameSource:     "Unknown",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityAutoScalingGroup:      "auto-scaling",
				entityattributes.AttributeEntityDeploymentEnvironment: "ec2:auto-scaling",
			},
		},
		{
			name:                          "DatapointAttributeEnvironmentOnly",
			metrics:                       generateDatapointMetrics(attributeDeploymentEnvironment, "test-environment"),
			mockServiceNameAndSource:      newMockGetServiceNameAndSource("test-service-name", "ClientIamRole"),
			mockGetEC2InfoFromEntityStore: newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", ""),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-service-name",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "ClientIamRole",
			},
		},
		{
			name:                          "DatapointAttributeServiceNameAndEnvironment",
			metrics:                       generateDatapointMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment"),
			mockGetEC2InfoFromEntityStore: newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", ""),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "Unknown",
			},
		},
		{
			name:                           "DatapointAttributeServiceAndEnvironmentNameUserConfiguration",
			checkDatapointAttributeRemoval: true,
			metrics:                        generateDatapointMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment", entityattributes.AttributeServiceNameSource, entityattributes.AttributeServiceNameSourceUserConfig, entityattributes.AttributeDeploymentEnvironmentSource, entityattributes.AttributeServiceNameSourceUserConfig),
			mockGetEC2InfoFromEntityStore:  newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", ""),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "UserConfiguration",
			},
			wantDatapointAttributes: map[string]any{},
		},
		{
			name:                           "DatapointAttributeServiceNameUserConfigurationAndUserEnvironment",
			checkDatapointAttributeRemoval: true,
			metrics:                        generateDatapointMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment", entityattributes.AttributeServiceNameSource, entityattributes.AttributeServiceNameSourceUserConfig),
			mockGetEC2InfoFromEntityStore:  newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", ""),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "UserConfiguration",
			},
			wantDatapointAttributes: map[string]any{
				attributeDeploymentEnvironment: "test-environment",
			},
		},
		{
			name:                           "DatapointAttributeEnvironmentNameUserConfigurationAndUserServiceName",
			checkDatapointAttributeRemoval: true,
			metrics:                        generateDatapointMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment", entityattributes.AttributeDeploymentEnvironmentSource, entityattributes.AttributeServiceNameSourceUserConfig),
			mockGetEC2InfoFromEntityStore:  newMockGetEC2InfoFromEntityStore("i-123456789", "0123456789012", ""),
			want: map[string]any{
				entityattributes.AttributeEntityType:                  "Service",
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				entityattributes.AttributeEntityPlatformType:          "AWS::EC2",
				entityattributes.AttributeEntityInstanceID:            "i-123456789",
				entityattributes.AttributeEntityAwsAccountId:          "0123456789012",
				entityattributes.AttributeEntityServiceNameSource:     "Unknown",
			},
			wantDatapointAttributes: map[string]any{
				attributeServiceName: "test-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make copy of original functions to use as resets later to prevent failing test when tests are ran in bulk
			resetServiceNameSource := getServiceNameSource
			if tt.mockServiceNameAndSource != nil {
				getServiceNameSource = tt.mockServiceNameAndSource
			}
			if tt.mockGetEC2InfoFromEntityStore != nil {
				getEC2InfoFromEntityStore = tt.mockGetEC2InfoFromEntityStore
			}
			p := newAwsEntityProcessor(&Config{ScrapeDatapointAttribute: true, EntityType: attributeService}, logger)
			p.config.Platform = config.ModeEC2
			_, err := p.processMetrics(ctx, tt.metrics)
			assert.NoError(t, err)
			rm := tt.metrics.ResourceMetrics()
			if rm.Len() > 0 {
				assert.Equal(t, tt.want, rm.At(0).Resource().Attributes().AsRaw())
			}
			if tt.checkDatapointAttributeRemoval {
				assert.Equal(t, tt.wantDatapointAttributes, rm.At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().AsRaw())
			}
			getServiceNameSource = resetServiceNameSource
		})
	}
}

func generateMetrics(resourceAttrs ...string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	generateResource(md, resourceAttrs...)
	return md
}

func generateMetricsWithTwoResources() pmetric.Metrics {
	md := pmetric.NewMetrics()
	generateResource(md, attributeAwsLogGroupNames, "test-log-group1", attributeServiceName, "test-service1")
	generateResource(md, attributeAwsLogGroupNames, "test-log-group2", attributeServiceName, "test-service2")
	return md
}

func generateDatapointMetrics(datapointAttrs ...string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	generateDatapoints(md, datapointAttrs...)
	return md
}

func generateResource(md pmetric.Metrics, resourceAttrs ...string) {
	attrs := md.ResourceMetrics().AppendEmpty().Resource().Attributes()
	for i := 0; i < len(resourceAttrs); i += 2 {
		attrs.PutStr(resourceAttrs[i], resourceAttrs[i+1])
	}
}

func generateDatapoints(md pmetric.Metrics, datapointAttrs ...string) {
	attrs := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty().Attributes()
	for i := 0; i < len(datapointAttrs); i += 2 {
		attrs.PutStr(datapointAttrs[i], datapointAttrs[i+1])
	}
}
