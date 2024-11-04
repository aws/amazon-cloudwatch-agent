// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sattributescraper

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
)

var (
	namespaceAllowlist = []string{
		semconv.AttributeK8SNamespaceName,
	}

	// these kubernetes resource attributes are set by the openTelemtry operator
	// see the code referecnes from upstream:
	// * https://github.com/open-telemetry/opentelemetry-operator/blame/main/pkg/instrumentation/sdk.go#L421
	workloadAllowlist = []string{
		semconv.AttributeK8SDeploymentName,
		semconv.AttributeK8SReplicaSetName,
		semconv.AttributeK8SStatefulSetName,
		semconv.AttributeK8SDaemonSetName,
		semconv.AttributeK8SCronJobName,
		semconv.AttributeK8SJobName,
		semconv.AttributeK8SPodName,
		semconv.AttributeK8SContainerName,
	}
	nodeAllowlist = []string{
		semconv.AttributeK8SNodeName,
	}
)

type K8sAttributeScraper struct {
	Cluster   string
	Namespace string
	Workload  string
	Node      string
}

func NewK8sAttributeScraper(clusterName string) *K8sAttributeScraper {
	return &K8sAttributeScraper{
		Cluster: clusterName,
	}
}

func (e *K8sAttributeScraper) Scrape(rm pcommon.Resource) {
	resourceAttrs := rm.Attributes()
	e.scrapeNamespace(resourceAttrs)
	e.scrapeWorkload(resourceAttrs)
	e.scrapeNode(resourceAttrs)
}

func (e *K8sAttributeScraper) scrapeNamespace(p pcommon.Map) {
	for _, namespace := range namespaceAllowlist {
		if namespaceAttr, ok := p.Get(namespace); ok {
			e.Namespace = namespaceAttr.Str()
			return
		}
	}
}

func (e *K8sAttributeScraper) scrapeWorkload(p pcommon.Map) {
	for _, workload := range workloadAllowlist {
		if workloadAttr, ok := p.Get(workload); ok {
			e.Workload = workloadAttr.Str()
			return
		}
	}

}

func (e *K8sAttributeScraper) scrapeNode(p pcommon.Map) {
	for _, node := range nodeAllowlist {
		if nodeAttr, ok := p.Get(node); ok {
			e.Node = nodeAttr.Str()
			return
		}
	}
}

func (e *K8sAttributeScraper) Reset() {
	*e = K8sAttributeScraper{
		Cluster: e.Cluster,
	}
}
