// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcedetectionprocessor

import (
	"slices"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var appendDimensionsKey = common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)

// legacyToOTel maps legacy ${aws:*} placeholders to OTel resource attribute names.
// Used for backward compatibility when EC2 eventually cuts over to resourcedetection.
var legacyToOTel = map[string]string{
	"${aws:InstanceId}":   "host.id",
	"${aws:InstanceType}": "host.type",
	"${aws:ImageId}":      "host.image.id",
}

type translator struct {
	name      string
	detectors []string
	factory   processor.Factory
}

var _ common.Translator[component.Config, component.ID] = (*translator)(nil)

func NewTranslator(detectors []string) common.Translator[component.Config, component.ID] {
	return NewTranslatorWithName("resourcedetection", detectors)
}

func NewTranslatorWithName(name string, detectors []string) common.Translator[component.Config, component.ID] {
	return &translator{
		name:      name,
		detectors: detectors,
		factory:   resourcedetectionprocessor.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a resourcedetection processor config.
// It reads append_dimensions from the JSON config and enables only the
// referenced resource attributes in each detector.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*resourcedetectionprocessor.Config)
	cfg.Detectors = t.detectors
	cfg.Timeout = 5 * time.Second
	cfg.Override = false

	requested := collectRequestedAttributes(conf)
	if slices.Contains(t.detectors, "azure") {
		configureAzureAttributes(cfg, requested)
	}

	return cfg, nil
}

// collectRequestedAttributes parses append_dimensions values and returns
// the set of OTel resource attribute names that are referenced.
// Supports both:
//   - "${host.id}" — OTel attribute name directly
//   - "${aws:InstanceId}" — legacy format, mapped to OTel attribute
func collectRequestedAttributes(conf *confmap.Conf) map[string]bool {
	requested := make(map[string]bool)
	if conf == nil || !conf.IsSet(appendDimensionsKey) {
		return requested
	}

	dims := conf.Get(appendDimensionsKey)
	dimMap, ok := dims.(map[string]interface{})
	if !ok {
		return requested
	}

	for _, v := range dimMap {
		vStr, ok := v.(string)
		if !ok {
			continue
		}
		if strings.HasPrefix(vStr, "${") && strings.HasSuffix(vStr, "}") {
			if otelAttr, exists := legacyToOTel[vStr]; exists {
				requested[otelAttr] = true
			} else {
				requested[vStr[2:len(vStr)-1]] = true
			}
		}
	}

	return requested
}

func configureAzureAttributes(cfg *resourcedetectionprocessor.Config, requested map[string]bool) {
	ra := &cfg.DetectorConfig.AzureConfig.ResourceAttributes
	ra.AzureResourcegroupName.Enabled = requested["azure.resourcegroup.name"]
	ra.AzureVMName.Enabled = requested["azure.vm.name"]
	ra.AzureVMScalesetName.Enabled = requested["azure.vm.scaleset.name"]
	ra.AzureVMSize.Enabled = requested["azure.vm.size"]
	ra.CloudAccountID.Enabled = requested["cloud.account.id"]
	ra.CloudPlatform.Enabled = requested["cloud.platform"]
	ra.CloudProvider.Enabled = requested["cloud.provider"]
	ra.CloudRegion.Enabled = requested["cloud.region"]
	ra.HostID.Enabled = requested["host.id"]
	ra.HostName.Enabled = requested["host.name"]
}
