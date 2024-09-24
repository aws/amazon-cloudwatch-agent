// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/internal/entityattributes"
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

func TestProcessMetricsLogGroupAssociation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := newAwsEntityProcessor(&Config{}, logger)
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
	p := newAwsEntityProcessor(&Config{ClusterName: "test-cluster"}, logger)
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
			want:    map[string]entitystore.ServiceEnvironment{},
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
		name    string
		metrics pmetric.Metrics
		want    map[string]any
	}{
		{
			name:    "EmptyMetrics",
			metrics: pmetric.NewMetrics(),
			want:    map[string]any{},
		},
		{
			name:    "ResourceAttributeServiceNameOnly",
			metrics: generateMetrics(attributeServiceName, "test-service"),
			want: map[string]any{
				entityattributes.AttributeEntityServiceName: "test-service",
				attributeServiceName:                        "test-service",
			},
		},
		{
			name:    "ResourceAttributeEnvironmentOnly",
			metrics: generateMetrics(attributeDeploymentEnvironment, "test-environment"),
			want: map[string]any{
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				attributeDeploymentEnvironment:                        "test-environment",
			},
		},
		{
			name:    "ResourceAttributeServiceNameAndEnvironment",
			metrics: generateMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment"),
			want: map[string]any{
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
				attributeServiceName:                                  "test-service",
				attributeDeploymentEnvironment:                        "test-environment",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newAwsEntityProcessor(&Config{}, logger)
			_, err := p.processMetrics(ctx, tt.metrics)
			assert.NoError(t, err)
			rm := tt.metrics.ResourceMetrics()
			if rm.Len() > 0 {
				assert.Equal(t, tt.want, rm.At(0).Resource().Attributes().AsRaw())
			}
		})
	}
}

func TestProcessMetricsDatapointAttributeScraping(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()
	tests := []struct {
		name    string
		metrics pmetric.Metrics
		want    map[string]any
	}{
		{
			name:    "EmptyMetrics",
			metrics: pmetric.NewMetrics(),
			want:    map[string]any{},
		},
		{
			name:    "DatapointAttributeServiceNameOnly",
			metrics: generateDatapointMetrics(attributeServiceName, "test-service"),
			want: map[string]any{
				entityattributes.AttributeEntityServiceName: "test-service",
			},
		},
		{
			name:    "DatapointAttributeEnvironmentOnly",
			metrics: generateDatapointMetrics(attributeDeploymentEnvironment, "test-environment"),
			want: map[string]any{
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
			},
		},
		{
			name:    "DatapointAttributeServiceNameAndEnvironment",
			metrics: generateDatapointMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment"),
			want: map[string]any{
				entityattributes.AttributeEntityServiceName:           "test-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "test-environment",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newAwsEntityProcessor(&Config{ScrapeDatapointAttribute: true}, logger)
			_, err := p.processMetrics(ctx, tt.metrics)
			assert.NoError(t, err)
			rm := tt.metrics.ResourceMetrics()
			if rm.Len() > 0 {
				assert.Equal(t, tt.want, rm.At(0).Resource().Attributes().AsRaw())
			}
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
