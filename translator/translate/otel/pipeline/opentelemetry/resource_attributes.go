// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
)

// reservedResourceAttributeKeys are attributes the agent manages internally for
// log routing; customers must not override them via resource_attributes.
var reservedResourceAttributeKeys = []string{
	"aws.log.group.name",
	"aws.log.stream.name",
	"aws.log.source",
}

// resourceAttributesProcessor returns a resource processor that upserts the
// customer-supplied opentelemetry.resource_attributes onto every record, or nil
// if none are configured. Callers place it at the front of the processor list so
// the attributes are present before any downstream processing.
//
// Note: it runs before resourcedetection (override: true), so for keys the agent
// also auto-detects (e.g. cloud.region, host.id) the detected value wins. This is
// intentional; the field is for adding attributes the agent does not detect.
func resourceAttributesProcessor(conf *confmap.Conf) common.ComponentTranslator {
	if conf == nil {
		return nil
	}
	attrs := common.GetStringMap(conf, common.OtelResourceAttributesKey)
	if len(attrs) == 0 {
		return nil
	}
	return resourceprocessor.NewTranslator(
		common.WithName(common.OpenTelemetryKey),
		resourceprocessor.WithAttributes(attrs),
		resourceprocessor.WithReservedKeys(reservedResourceAttributeKeys...),
	)
}
