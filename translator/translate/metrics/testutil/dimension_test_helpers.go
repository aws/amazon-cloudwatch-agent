// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package testutil

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

// MockEC2Metadata sets up a mock EC2 metadata provider for tests that resolve
// ${aws:*} placeholders in append_dimensions. Returns a cleanup function that
// restores the original provider.
//
// Usage:
//
//	cleanup := testutil.MockEC2Metadata(&util.Metadata{
//	    InstanceID: "i-1234567890abcdef0", InstanceType: "t3.medium",
//	})
//	defer cleanup()
func MockEC2Metadata(m *util.Metadata) func() {
	original := util.Ec2MetadataInfoProvider
	util.Ec2MetadataInfoProvider = func() *util.Metadata { return m }
	return func() { util.Ec2MetadataInfoProvider = original }
}

// UnmarshalAndApplyRule is a convenience for the common test pattern of
// unmarshaling a JSON config string and passing it through a rule's ApplyRule.
func UnmarshalAndApplyRule(t *testing.T, jsonInput string, rule interface {
	ApplyRule(interface{}) (string, interface{})
}) (string, interface{}) {
	t.Helper()
	var input interface{}
	require.NoError(t, json.Unmarshal([]byte(jsonInput), &input))
	return rule.ApplyRule(input)
}

// AssertDimensionsEqual validates that the tags/dimensions map within an
// ApplyRule result matches the expected key-value pairs. It navigates the
// common result shape: []interface{} -> [0] -> map["tags"].
func AssertDimensionsEqual(t *testing.T, actual interface{}, expected map[string]interface{}) {
	t.Helper()
	slice, ok := actual.([]interface{})
	require.True(t, ok, "expected result to be []interface{}")
	require.NotEmpty(t, slice, "expected non-empty result slice")

	m, ok := slice[0].(map[string]interface{})
	require.True(t, ok, "expected first element to be map[string]interface{}")

	tags, ok := m["tags"].(map[string]interface{})
	require.True(t, ok, "expected 'tags' key to be map[string]interface{}")

	for k, v := range expected {
		assert.Equal(t, v, tags[k], "dimension %q mismatch", k)
	}
}
