// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/resourcestore"
)

type mockResourceStore struct {
	entries []resourceStoreEntry
}

type resourceStoreEntry struct {
	logGroupName    resourcestore.LogGroupName
	serviceName     string
	environmentName string
}

func newMockResourceStore() *mockResourceStore {
	return &mockResourceStore{
		entries: make([]resourceStoreEntry, 0),
	}
}

func newAddToMockResourceStore(rs *mockResourceStore) func(resourcestore.LogGroupName, string, string) {
	return func(logGroupName resourcestore.LogGroupName, serviceName string, environmentName string) {
		rs.entries = append(rs.entries, resourceStoreEntry{
			logGroupName:    logGroupName,
			serviceName:     serviceName,
			environmentName: environmentName,
		})
	}
}

func TestProcessMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	p := newAwsEntityProcessor(logger)
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
		want    []resourceStoreEntry
	}{
		{
			name:    "EmptyMetrics",
			metrics: pmetric.NewMetrics(),
			want:    []resourceStoreEntry{},
		},
		{
			name:    "NoLogGroupNames",
			metrics: generateMetrics(attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment"),
			want:    []resourceStoreEntry{},
		},
		{
			name:    "NoServiceOrEnvironment",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group"),
			want:    []resourceStoreEntry{},
		},
		{
			name:    "LogGroupNameAndService",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group", attributeServiceName, "test-service"),
			want:    []resourceStoreEntry{{logGroupName: "test-log-group", serviceName: "test-service"}},
		},
		{
			name:    "LogGroupNameAndEnvironment",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group", attributeDeploymentEnvironment, "test-environment"),
			want:    []resourceStoreEntry{{logGroupName: "test-log-group", environmentName: "test-environment"}},
		},
		{
			name:    "LogGroupNameAndServiceAndEnvironment",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group", attributeServiceName, "test-service", attributeDeploymentEnvironment, "test-environment"),
			want:    []resourceStoreEntry{{logGroupName: "test-log-group", serviceName: "test-service", environmentName: "test-environment"}},
		},
		{
			name:    "TwoLogGroupNames",
			metrics: generateMetrics(attributeAwsLogGroupNames, "test-log-group1&test-log-group2", attributeServiceName, "test-service"),
			want: []resourceStoreEntry{
				{logGroupName: "test-log-group1", serviceName: "test-service"},
				{logGroupName: "test-log-group2", serviceName: "test-service"},
			},
		},
		{
			name:    "EmptyLogGroupNames",
			metrics: generateMetrics(attributeAwsLogGroupNames, "&&test-log-group1&&test-log-group2&&", attributeServiceName, "test-service"),
			want: []resourceStoreEntry{
				{logGroupName: "test-log-group1", serviceName: "test-service"},
				{logGroupName: "test-log-group2", serviceName: "test-service"},
			},
		},
		{
			name:    "TwoResourceMetrics",
			metrics: generateMetricsWithTwoResources(),
			want: []resourceStoreEntry{
				{logGroupName: "test-log-group1", serviceName: "test-service1"},
				{logGroupName: "test-log-group2", serviceName: "test-service2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newMockResourceStore()
			addToResourceStore = newAddToMockResourceStore(rs)
			_, err := p.processMetrics(ctx, tt.metrics)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, rs.entries)
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

func generateResource(md pmetric.Metrics, resourceAttrs ...string) {
	attrs := md.ResourceMetrics().AppendEmpty().Resource().Attributes()
	for i := 0; i < len(resourceAttrs); i += 2 {
		attrs.PutStr(resourceAttrs[i], resourceAttrs[i+1])
	}
}
