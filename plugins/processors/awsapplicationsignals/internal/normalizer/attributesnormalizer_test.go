// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package normalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	deprecatedsemconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"

	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
)

func TestRenameAttributes_for_metric(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	normalizer := NewAttributesNormalizer(logger)

	// test for metric
	// Create a pcommon.Map with some attributes
	attributes := pcommon.NewMap()
	for originalKey, replacementKey := range attributesRenamingForMetric {
		attributes.PutStr(originalKey, replacementKey+"-value")
	}

	resourceAttributes := pcommon.NewMap()
	// Call the process method
	normalizer.renameAttributes(attributes, resourceAttributes, false)

	// Check that the original key has been removed
	for originalKey := range attributesRenamingForMetric {
		if _, ok := attributes.Get(originalKey); ok {
			t.Errorf("originalKey was not removed")
		}
	}

	// Check that the new key has the correct value
	for _, replacementKey := range attributesRenamingForMetric {
		assertStringAttributeEqual(t, attributes, replacementKey, replacementKey+"-value")
	}
}

func TestRenameAttributes_for_trace(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	normalizer := NewAttributesNormalizer(logger)

	// test for trace
	// Create a pcommon.Map with some attributes
	resourceAttributes := pcommon.NewMap()
	for originalKey, replacementKey := range resourceAttributesRenamingForTrace {
		resourceAttributes.PutStr(originalKey, replacementKey+"-value")
	}
	resourceAttributes.PutStr("host.id", "i-01ef7d37f42caa168")

	attributes := pcommon.NewMap()
	// Call the process method
	normalizer.renameAttributes(attributes, resourceAttributes, true)

	// Check that the original key has been removed
	for originalKey := range resourceAttributesRenamingForTrace {
		if _, ok := resourceAttributes.Get(originalKey); ok {
			t.Errorf("originalKey was not removed")
		}
	}

	// Check that the new key has the correct value
	for _, replacementKey := range resourceAttributesRenamingForTrace {
		assertStringAttributeEqual(t, resourceAttributes, replacementKey, replacementKey+"-value")
	}

	if value, ok := resourceAttributes.Get("K8s.Node"); !ok || value.AsString() != "i-01ef7d37f42caa168" {
		t.Errorf("replacementKey has incorrect value: got %v, want %v", value.AsString(), "i-01ef7d37f42caa168")
	}
}

func TestCopyResourceAttributesToAttributes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	normalizer := NewAttributesNormalizer(logger)

	// Create a pcommon.Map for resourceAttributes with some attributes
	resourceAttributes := pcommon.NewMap()
	for resourceAttrKey, attrKey := range resourceToMetricAttributes {
		resourceAttributes.PutStr(resourceAttrKey, attrKey+"-value")
	}
	resourceAttributes.PutStr("host.id", "i-01ef7d37f42caa168")
	resourceAttributes.PutStr("aws.local.service", "test-app")

	// Create a pcommon.Map for attributes
	attributes := pcommon.NewMap()

	// Call the process method
	normalizer.copyResourceAttributesToAttributes(attributes, resourceAttributes, false)

	// Check that the attribute has been copied correctly
	for _, attrKey := range resourceToMetricAttributes {
		assertStringAttributeEqual(t, attributes, attrKey, attrKey+"-value")
	}

	assertStringAttributeEqual(t, attributes, "K8s.Node", "i-01ef7d37f42caa168")
	assertStringAttributeEqual(t, attributes, "aws.local.service", "test-app")
}

func TestTruncateAttributes(t *testing.T) {
	attributes := pcommon.NewMap()

	longValue := make([]byte, 300)
	for i := 0; i < 300; i++ {
		longValue[i] = 'a'
	}
	longStringValue := string(longValue)
	for key, _ := range attributesRenamingForMetric {
		attributes.PutStr(key, longStringValue)
	}

	truncateAttributesByLength(attributes)

	val, _ := attributes.Get(attr.AWSLocalEnvironment)
	assert.True(t, len(val.Str()) == maxEnvironmentLength)
	val, _ = attributes.Get(attr.AWSRemoteEnvironment)
	assert.True(t, len(val.Str()) == maxEnvironmentLength)
	val, _ = attributes.Get(attr.AWSLocalService)
	assert.True(t, len(val.Str()) == maxServiceNameLength)
	val, _ = attributes.Get(attr.AWSRemoteService)
	assert.True(t, len(val.Str()) == maxServiceNameLength)
	val, _ = attributes.Get(attr.AWSRemoteResourceIdentifier)
	assert.True(t, len(val.Str()) == 300)
}

func Test_attributesNormalizer_appendNewAttributes(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	completeResourceAttributes := pcommon.NewMap()
	completeResourceAttributes.PutStr(semconv.AttributeTelemetrySDKName, "opentelemetry")
	completeResourceAttributes.PutStr(deprecatedsemconv.AttributeTelemetryAutoVersion, "0.0.1 auto")
	completeResourceAttributes.PutStr(semconv.AttributeTelemetrySDKVersion, "0.0.1 test")
	completeResourceAttributes.PutStr(semconv.AttributeTelemetrySDKLanguage, "go")

	incompleteResourceAttributes := pcommon.NewMap()
	incompleteResourceAttributes.PutStr(semconv.AttributeTelemetrySDKName, "opentelemetry")
	incompleteResourceAttributes.PutStr(semconv.AttributeTelemetrySDKVersion, "0.0.1 test")

	tests := []struct {
		name                   string
		attributes             pcommon.Map
		resourceAttributes     pcommon.Map
		isTrace                bool
		expectedAttributeValue string
	}{
		{
			"testAppendNoAttributesToTrace",
			pcommon.NewMap(),
			completeResourceAttributes,
			true,
			"",
		}, {
			"testAppendAttributesToMetricWithValuesFound",
			pcommon.NewMap(),
			completeResourceAttributes,
			false,
			"opentelemetry,0.0.1auto,go,Auto",
		},
		{
			"testAppendAttributesToMetricWithSomeValuesMissing",
			pcommon.NewMap(),
			incompleteResourceAttributes,
			false,
			"opentelemetry,0.0.1test,-,Manual",
		},
		{

			"testAppendAttributesToMetricWithAllValuesMissing",
			pcommon.NewMap(),
			pcommon.NewMap(),
			false,
			"-,-,-,Manual",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &attributesNormalizer{
				logger: logger,
			}
			n.normalizeTelemetryAttributes(tt.attributes, tt.resourceAttributes, tt.isTrace)

			if value, ok := tt.attributes.Get("Telemetry.SDK"); !ok {
				if !tt.isTrace {
					t.Errorf("attribute is not found.")
				}
			} else {
				if tt.isTrace {
					t.Errorf("unexpected attribute is found.")
				}
				assert.Equal(t, tt.expectedAttributeValue, value.Str())
			}
		})
	}
}

func TestRenameAttributes_AWSRemoteDbUser_for_metric(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	normalizer := NewAttributesNormalizer(logger)

	attributes := pcommon.NewMap()
	attributes.PutStr(attr.AWSRemoteDbUser, "remoteDbUser-value")

	resourceAttributes := pcommon.NewMap()
	normalizer.renameAttributes(attributes, resourceAttributes, false)

	if _, ok := attributes.Get(attr.AWSRemoteDbUser); ok {
		t.Errorf("AWSRemoteDbUser was not removed")
	}

	if value, ok := attributes.Get("RemoteDbUser"); !ok || value.AsString() != "remoteDbUser-value" {
		t.Errorf("MetricAttributeRemoteDbUser has incorrect value: got %v, want %v", value.AsString(), "remoteDbUser-value")
	}
}

func TestTruncateAttributes_AWSRemoteDbUser(t *testing.T) {
	attributes := pcommon.NewMap()

	longValue := make([]byte, 300)
	for i := 0; i < 300; i++ {
		longValue[i] = 'a'
	}
	longStringValue := string(longValue)
	attributes.PutStr(attr.AWSRemoteDbUser, longStringValue)

	truncateAttributesByLength(attributes)

	val, _ := attributes.Get(attr.AWSRemoteDbUser)
	assert.True(t, len(val.Str()) <= defaultMetricAttributeLength)
}

func TestRenameAttributes_AWSRemoteResourceCfnIdentifier_for_metric(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	normalizer := NewAttributesNormalizer(logger)

	attributes := pcommon.NewMap()
	attributes.PutStr(attr.AWSRemoteResourceCfnPrimaryIdentifier, "arn:123:abc-value")

	resourceAttributes := pcommon.NewMap()
	normalizer.renameAttributes(attributes, resourceAttributes, false)

	if _, ok := attributes.Get(attr.AWSRemoteResourceCfnPrimaryIdentifier); ok {
		t.Errorf("AWSRemoteResourceCfnPrimaryIdentifier was not removed")
	}

	if value, ok := attributes.Get("RemoteResourceCfnPrimaryIdentifier"); !ok || value.AsString() != "arn:123:abc-value" {
		t.Errorf("RemoteResourceCfnPrimaryIdentifier has incorrect value: got %v, want %v", value.AsString(), "arn:123:abc-value")
	}
}

func assertStringAttributeEqual(t *testing.T, attributes pcommon.Map, attrKey, attrVal string) {
	if val, ok := attributes.Get(attrKey); ok {
		if val.AsString() != attrVal {
			t.Errorf("Attribute was not copied correctly: got %v, want %v", val.AsString(), attrVal)
		}
	} else {
		t.Errorf("Attribute %s is not found", attrKey)
	}
}
