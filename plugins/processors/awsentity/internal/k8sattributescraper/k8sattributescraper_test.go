// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sattributescraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
)

func TestNewK8sAttributeScraper(t *testing.T) {
	scraper := NewK8sAttributeScraper("test")
	assert.Equal(t, "test", scraper.Cluster)
	assert.Empty(t, scraper.Namespace)
	assert.Empty(t, scraper.Workload)
	assert.Empty(t, scraper.Node)
}

func Test_k8sattributescraper_Scrape(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		args        pcommon.Resource
		podMeta     k8sclient.PodMetadata
		want        *K8sAttributeScraper
	}{
		{
			name:        "Empty",
			clusterName: "",
			args:        pcommon.NewResource(),
			podMeta:     k8sclient.PodMetadata{},
			want:        &K8sAttributeScraper{},
		},
		{
			name:        "ClusterOnly",
			clusterName: "test-cluster",
			args:        pcommon.NewResource(),
			podMeta:     k8sclient.PodMetadata{},
			want: &K8sAttributeScraper{
				Cluster: "test-cluster",
			},
		},
		{
			name:        "AllAppSignalAttributes",
			clusterName: "test-cluster",
			args: generateResourceMetrics(
				semconv.AttributeK8SNamespaceName, "test-namespace",
				semconv.AttributeK8SDeploymentName, "test-workload",
				semconv.AttributeK8SNodeName, "test-node",
			),
			podMeta: k8sclient.PodMetadata{},
			want: &K8sAttributeScraper{
				Cluster:   "test-cluster",
				Namespace: "test-namespace",
				Workload:  "test-workload",
				Node:      "test-node",
			},
		},
		{
			name:        "PodMetadataOnly",
			clusterName: "my-cluster",
			args:        pcommon.NewResource(),
			podMeta: k8sclient.PodMetadata{
				Namespace: "podmeta-namespace",
				Workload:  "podmeta-workload",
				Node:      "podmeta-node",
			},
			want: &K8sAttributeScraper{
				Cluster:   "my-cluster",
				Namespace: "podmeta-namespace",
				Workload:  "podmeta-workload",
				Node:      "podmeta-node",
			},
		},
		{
			name:        "MixedResourceAndPodMeta",
			clusterName: "test-cluster",
			args: generateResourceMetrics(
				semconv.AttributeK8SNamespaceName, "resource-namespace",
				semconv.AttributeK8SDeploymentName, "resource-workload",
				semconv.AttributeK8SNodeName, "resource-node",
			),
			podMeta: k8sclient.PodMetadata{
				Workload: "podmeta-workload",
			},
			want: &K8sAttributeScraper{
				Cluster:   "test-cluster",
				Namespace: "resource-namespace",
				Workload:  "podmeta-workload",
				Node:      "resource-node",
			},
		},
		{
			name:        "PodMetaOverridesAllResourceAttrs",
			clusterName: "test-cluster",
			args: generateResourceMetrics(
				semconv.AttributeK8SNamespaceName, "resource-namespace",
				semconv.AttributeK8SDeploymentName, "resource-workload",
				semconv.AttributeK8SNodeName, "resource-node",
			),
			podMeta: k8sclient.PodMetadata{
				Namespace: "override-namespace",
				Workload:  "override-workload",
				Node:      "override-node",
			},
			want: &K8sAttributeScraper{
				Cluster:   "test-cluster",
				Namespace: "override-namespace",
				Workload:  "override-workload",
				Node:      "override-node",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewK8sAttributeScraper(tt.clusterName)
			e.Scrape(tt.args, tt.podMeta)
			assert.Equal(t, tt.want, e)
		})
	}
}

func Test_k8sattributescraper_reset(t *testing.T) {
	type fields struct {
		Cluster   string
		Namespace string
		Workload  string
		Node      string
	}
	tests := []struct {
		name   string
		fields fields
		want   *K8sAttributeScraper
	}{
		{
			name:   "Empty",
			fields: fields{},
			want:   &K8sAttributeScraper{},
		},
		{
			name: "ClusterExists",
			fields: fields{
				Cluster: "test-cluster",
			},
			want: &K8sAttributeScraper{
				Cluster: "test-cluster",
			},
		},
		{
			name: "MultipleAttributeExists",
			fields: fields{
				Cluster:   "test-cluster",
				Namespace: "test-namespace",
				Workload:  "test-workload",
				Node:      "test-node",
			},
			want: &K8sAttributeScraper{
				Cluster: "test-cluster",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &K8sAttributeScraper{
				Cluster:   tt.fields.Cluster,
				Namespace: tt.fields.Namespace,
				Workload:  tt.fields.Workload,
				Node:      tt.fields.Node,
			}
			e.Reset()
			assert.Equal(t, tt.want, e)
		})
	}
}

func Test_k8sattributescraper_scrapeNamespace(t *testing.T) {
	tests := []struct {
		name  string
		nsArg string
		args  pcommon.Map
		want  string
	}{
		{
			name: "Empty",
			args: getAttributeMap(map[string]any{"": ""}),
			want: "",
		},
		{
			name:  "DirectOverride",
			nsArg: "direct-namespace",
			args:  getAttributeMap(map[string]any{semconv.AttributeK8SNamespaceName: "namespace-name"}),
			want:  "direct-namespace",
		},
		{
			name: "AppSignalNodeExists",
			args: getAttributeMap(map[string]any{semconv.AttributeK8SNamespaceName: "namespace-name"}),
			want: "namespace-name",
		},
		{
			name: "NonmatchingNamespace",
			args: getAttributeMap(map[string]any{"namespace": "namespace-name"}),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &K8sAttributeScraper{}
			e.scrapeNamespace(tt.args, tt.nsArg)
			assert.Equal(t, tt.want, e.Namespace)
		})
	}
}

func Test_k8sattributescraper_scrapeNode(t *testing.T) {
	tests := []struct {
		name  string
		ndArg string
		args  pcommon.Map
		want  string
	}{
		{
			name: "Empty",
			args: getAttributeMap(map[string]any{"": ""}),
			want: "",
		},
		{
			name:  "DirectOverride",
			ndArg: "direct-node",
			args:  getAttributeMap(map[string]any{semconv.AttributeK8SNodeName: "resource-node"}),
			want:  "direct-node",
		},
		{
			name: "AppsignalNodeExists",
			args: getAttributeMap(map[string]any{semconv.AttributeK8SNodeName: "node-name"}),
			want: "node-name",
		},
		{
			name: "NonmatchingNode",
			args: getAttributeMap(map[string]any{"node": "node-name"}),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &K8sAttributeScraper{}
			e.scrapeNode(tt.args, tt.ndArg)
			assert.Equal(t, tt.want, e.Node)
		})
	}
}

func Test_k8sattributescraper_scrapeWorkload(t *testing.T) {
	tests := []struct {
		name  string
		wlArg string
		args  pcommon.Map
		want  string
	}{
		{
			name: "Empty",
			args: getAttributeMap(map[string]any{"": ""}),
			want: "",
		},
		{
			name: "DeploymentWorkload",
			args: getAttributeMap(map[string]any{semconv.AttributeK8SDeploymentName: "test-deployment"}),
			want: "test-deployment",
		},
		{
			name: "DaemonsetWorkload",
			args: getAttributeMap(map[string]any{semconv.AttributeK8SDaemonSetName: "test-daemonset"}),
			want: "test-daemonset",
		},
		{
			name: "StatefulSetWorkload",
			args: getAttributeMap(map[string]any{semconv.AttributeK8SStatefulSetName: "test-statefulset"}),
			want: "test-statefulset",
		},
		{
			name: "ReplicaSetWorkload",
			args: getAttributeMap(map[string]any{semconv.AttributeK8SReplicaSetName: "test-replicaset"}),
			want: "test-replicaset",
		},
		{
			name: "ContainerWorkload",
			args: getAttributeMap(map[string]any{semconv.AttributeK8SContainerName: "test-container"}),
			want: "test-container",
		},
		{
			name:  "DirectOverride",
			wlArg: "direct-workload",
			args:  getAttributeMap(map[string]any{semconv.AttributeK8SDeploymentName: "resource-workload"}),
			want:  "direct-workload",
		},
		{
			name: "MultipleWorkloads",
			args: getAttributeMap(map[string]any{
				semconv.AttributeK8SDeploymentName: "test-deployment",
				semconv.AttributeK8SContainerName:  "test-container",
			}),
			want: "test-deployment",
		},
		{
			name: "NoArgNoResource",
			args: getAttributeMap(map[string]any{"foo": "bar"}),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &K8sAttributeScraper{}
			e.scrapeWorkload(tt.args, tt.wlArg)
			assert.Equal(t, tt.want, e.Workload)
		})
	}
}

func getAttributeMap(attributes map[string]any) pcommon.Map {
	attrMap := pcommon.NewMap()
	attrMap.FromRaw(attributes)
	return attrMap
}

func generateResourceMetrics(resourceAttrs ...string) pcommon.Resource {
	md := pmetric.NewMetrics()
	generateResource(md, resourceAttrs...)
	return md.ResourceMetrics().At(0).Resource()
}

func generateResource(md pmetric.Metrics, resourceAttrs ...string) {
	attrs := md.ResourceMetrics().AppendEmpty().Resource().Attributes()
	for i := 0; i < len(resourceAttrs); i += 2 {
		attrs.PutStr(resourceAttrs[i], resourceAttrs[i+1])
	}
}
