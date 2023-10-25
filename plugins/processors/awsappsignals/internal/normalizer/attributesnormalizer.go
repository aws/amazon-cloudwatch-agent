// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package normalizer

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type attributesNormalizer struct {
	logger *zap.Logger
}

var renameMapForMetric = map[string]string{
	"aws.local.service":    "Service",
	"aws.local.operation":  "Operation",
	"aws.remote.service":   "RemoteService",
	"aws.remote.operation": "RemoteOperation",
	"aws.remote.target":    "RemoteTarget",
}

var renameMapForTrace = map[string]string{
	// these kubernetes resource attributes are set by the openTelemtry operator
	// see the code referecnes from upstream:
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L245
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L305C43-L305C43
	"k8s.deployment.name":  "K8s.Workload",
	"k8s.statefulset.name": "K8s.Workload",
	"k8s.daemonset.name":   "K8s.Workload",
	"k8s.job.name":         "K8s.Workload",
	"k8s.cronjob.name":     "K8s.Workload",
	"k8s.pod.name":         "K8s.Pod",
}

var copyMapForMetric = map[string]string{
	// these kubernetes resource attributes are set by the openTelemtry operator
	// see the code referecnes from upstream:
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L245
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L305C43-L305C43
	"k8s.deployment.name":  "K8s.Workload",
	"k8s.statefulset.name": "K8s.Workload",
	"k8s.daemonset.name":   "K8s.Workload",
	"k8s.job.name":         "K8s.Workload",
	"k8s.cronjob.name":     "K8s.Workload",
	"k8s.pod.name":         "K8s.Pod",
}

func NewAttributesNormalizer(logger *zap.Logger) *attributesNormalizer {
	return &attributesNormalizer{
		logger: logger,
	}
}

func (n *attributesNormalizer) Process(attributes, resourceAttributes pcommon.Map, isTrace bool) error {
	n.copyResourceAttributesToAttributes(attributes, resourceAttributes, isTrace)
	n.renameAttributes(attributes, resourceAttributes, isTrace)
	return nil
}

func (n *attributesNormalizer) renameAttributes(attributes, resourceAttributes pcommon.Map, isTrace bool) {
	attrs := attributes
	renameMap := renameMapForMetric
	if isTrace {
		attrs = resourceAttributes
		renameMap = renameMapForTrace
	}

	rename(attrs, renameMap)
}

func (n *attributesNormalizer) copyResourceAttributesToAttributes(attributes, resourceAttributes pcommon.Map, isTrace bool) {
	if isTrace {
		return
	}
	for k, v := range copyMapForMetric {
		if resourceAttrValue, ok := resourceAttributes.Get(k); ok {
			// print some debug info when an attribute value is overwritten
			if originalAttrValue, ok := attributes.Get(k); ok {
				n.logger.Debug("attribute value is overwritten", zap.String("attribute", k), zap.String("original", originalAttrValue.AsString()), zap.String("new", resourceAttrValue.AsString()))
			}
			attributes.PutStr(v, resourceAttrValue.AsString())
			if k == "k8s.pod.name" {
				// only copy "host.id" from resource attributes to "K8s.Node" in attributesif the pod name is set
				if host, ok := resourceAttributes.Get("host.id"); ok {
					attributes.PutStr("K8s.Node", host.AsString())
				}
			}
		}
	}
}

func rename(attrs pcommon.Map, renameMap map[string]string) {
	for original, replacement := range renameMap {
		if value, ok := attrs.Get(original); ok {
			attrs.PutStr(replacement, value.AsString())
			attrs.Remove(original)
			if original == "k8s.pod.name" {
				// only rename host.id if the pod name is set
				if host, ok := attrs.Get("host.id"); ok {
					attrs.PutStr("K8s.Node", host.AsString())
					attrs.Remove("host.id")
				}
			}
		}
	}
}
