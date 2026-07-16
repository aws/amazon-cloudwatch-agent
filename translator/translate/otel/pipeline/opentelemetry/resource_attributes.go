// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
)

// resourceAttributesProcessor returns a resource processor that upserts the
// customer-supplied opentelemetry.resource_attributes onto every record, or nil
// if none are configured. Callers place it at the front of the processor list so
// the attributes are present before any downstream processing.
func resourceAttributesProcessor(conf *confmap.Conf) common.ComponentTranslator {
	if conf == nil {
		return nil
	}
	attrs := common.GetStringMap(conf, common.OtelResourceAttributesKey)
	if len(attrs) == 0 {
		return nil
	}
	return resourceprocessor.NewTranslator(common.WithName(common.OpenTelemetryKey), resourceprocessor.WithAttributes(attrs))
}
