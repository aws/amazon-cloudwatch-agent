// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package normalizer

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
	"go.uber.org/zap"

	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/internal/attributes"
)

type attributesNormalizer struct {
	logger *zap.Logger
}

var renameMapForMetric = map[string]string{
	attr.AWSLocalService:    "Service",
	attr.AWSLocalOperation:  "Operation",
	attr.AWSRemoteService:   "RemoteService",
	attr.AWSRemoteOperation: "RemoteOperation",
	attr.AWSRemoteTarget:    "RemoteTarget",
}

var renameMapForTrace = map[string]string{
	// these kubernetes resource attributes are set by the openTelemetry operator
	// see the code references from upstream:
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L245
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L305C43-L305C43
	attr.K8SDeploymentName:  "K8s.Workload",
	attr.K8SStatefulSetName: "K8s.Workload",
	attr.K8SDaemonSetName:   "K8s.Workload",
	attr.K8SJobName:         "K8s.Workload",
	attr.K8SCronJobName:     "K8s.Workload",
	attr.K8SPodName:         "K8s.Pod",
}

var copyMapForMetric = map[string]string{
	// these kubernetes resource attributes are set by the openTelemtry operator
	// see the code referecnes from upstream:
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L245
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L305C43-L305C43
	attr.K8SDeploymentName:  "K8s.Workload",
	attr.K8SStatefulSetName: "K8s.Workload",
	attr.K8SDaemonSetName:   "K8s.Workload",
	attr.K8SJobName:         "K8s.Workload",
	attr.K8SCronJobName:     "K8s.Workload",
	attr.K8SPodName:         "K8s.Pod",
}

const (
	instrumentationModeAuto   = "Auto"
	instrumentationModeManual = "Manual"
)

func NewAttributesNormalizer(logger *zap.Logger) *attributesNormalizer {
	return &attributesNormalizer{
		logger: logger,
	}
}

func (n *attributesNormalizer) Process(attributes, resourceAttributes pcommon.Map, isTrace bool) error {
	n.copyResourceAttributesToAttributes(attributes, resourceAttributes, isTrace)
	n.renameAttributes(attributes, resourceAttributes, isTrace)
	n.appendNewAttributes(attributes, resourceAttributes, isTrace)
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
			if k == attr.K8SPodName {
				// only copy "host.id" from resource attributes to "K8s.Node" in attributesif the pod name is set
				if host, ok := resourceAttributes.Get("host.id"); ok {
					attributes.PutStr("K8s.Node", host.AsString())
				}
			}
		}
	}
}

func (n *attributesNormalizer) appendNewAttributes(attributes, resourceAttributes pcommon.Map, isTrace bool) {
	if isTrace {
		return
	}

	var (
		sdkName        string
		sdkVersion     string
		sdkAutoVersion string
		sdkLang        string
	)
	sdkName, sdkVersion, sdkLang = "-", "-", "-"
	mode := instrumentationModeManual

	// TODO read telemetry.auto.version from telemetry.distro.* from v1.22
	resourceAttributes.Range(func(k string, v pcommon.Value) bool {
		switch k {
		case conventions.AttributeTelemetrySDKName:
			sdkName = strings.ReplaceAll(v.Str(), " ", "")
		case conventions.AttributeTelemetrySDKLanguage:
			sdkLang = strings.ReplaceAll(v.Str(), " ", "")
		case conventions.AttributeTelemetrySDKVersion:
			sdkVersion = strings.ReplaceAll(v.Str(), " ", "")
		case conventions.AttributeTelemetryAutoVersion:
			sdkAutoVersion = strings.ReplaceAll(v.Str(), " ", "")
		}
		return true
	})
	if sdkAutoVersion != "" {
		sdkVersion = sdkAutoVersion
		mode = instrumentationModeAuto
	}
	attributes.PutStr(attr.MetricAttributeSDKMetadata, fmt.Sprintf("%s,%s,%s,%s", sdkName, sdkVersion, sdkLang, mode))
}

func rename(attrs pcommon.Map, renameMap map[string]string) {
	for original, replacement := range renameMap {
		if value, ok := attrs.Get(original); ok {
			attrs.PutStr(replacement, value.AsString())
			attrs.Remove(original)
			if original == attr.K8SPodName {
				// only rename host.id if the pod name is set
				if host, ok := attrs.Get("host.id"); ok {
					attrs.PutStr("K8s.Node", host.AsString())
					attrs.Remove("host.id")
				}
			}
		}
	}
}
