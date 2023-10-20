// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package normalizer

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

func TestRenameAttributes_for_metric(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	normalizer := NewAttributesNormalizer(logger)

	// test for metric
	// Create a pcommon.Map with some attributes
	attributes := pcommon.NewMap()
	for originalKey, replacementKey := range renameMapForMetric {
		attributes.PutStr(originalKey, replacementKey+"-value")
	}

	resourceAttributes := pcommon.NewMap()
	// Call the process method
	normalizer.renameAttributes(attributes, resourceAttributes, false)

	// Check that the original key has been removed
	for originalKey := range renameMapForMetric {
		if _, ok := attributes.Get(originalKey); ok {
			t.Errorf("originalKey was not removed")
		}
	}

	// Check that the new key has the correct value
	for _, replacementKey := range renameMapForMetric {
		if value, ok := attributes.Get(replacementKey); !ok || value.AsString() != replacementKey+"-value" {
			t.Errorf("replacementKey has incorrect value: got %v, want %v", value.AsString(), replacementKey+"-value")
		}
	}
}

func TestRenameAttributes_for_trace(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	normalizer := NewAttributesNormalizer(logger)

	// test for trace
	// Create a pcommon.Map with some attributes
	resourceAttributes := pcommon.NewMap()
	for originalKey, replacementKey := range renameMapForTrace {
		resourceAttributes.PutStr(originalKey, replacementKey+"-value")
	}
	resourceAttributes.PutStr("host.id", "i-01ef7d37f42caa168")

	attributes := pcommon.NewMap()
	// Call the process method
	normalizer.renameAttributes(attributes, resourceAttributes, true)

	// Check that the original key has been removed
	for originalKey := range renameMapForTrace {
		if _, ok := resourceAttributes.Get(originalKey); ok {
			t.Errorf("originalKey was not removed")
		}
	}

	// Check that the new key has the correct value
	for _, replacementKey := range renameMapForTrace {
		if value, ok := resourceAttributes.Get(replacementKey); !ok || value.AsString() != replacementKey+"-value" {
			t.Errorf("replacementKey has incorrect value: got %v, want %v", value.AsString(), replacementKey+"-value")
		}
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
	for resourceAttrKey, attrKey := range copyMapForMetric {
		resourceAttributes.PutStr(resourceAttrKey, attrKey+"-value")
	}
	resourceAttributes.PutStr("host.id", "i-01ef7d37f42caa168")

	// Create a pcommon.Map for attributes
	attributes := pcommon.NewMap()

	// Call the process method
	normalizer.copyResourceAttributesToAttributes(attributes, resourceAttributes, false)

	// Check that the attribute has been copied correctly
	for _, attrKey := range copyMapForMetric {
		if value, ok := attributes.Get(attrKey); !ok || value.AsString() != attrKey+"-value" {
			t.Errorf("Attribute was not copied correctly: got %v, want %v", value.AsString(), attrKey+"-value")
		}
	}

	if value, ok := attributes.Get("K8s.Node"); !ok || value.AsString() != "i-01ef7d37f42caa168" {
		t.Errorf("Attribute was not copied correctly: got %v, want %v", value.AsString(), "i-01ef7d37f42caa168")
	}
}
