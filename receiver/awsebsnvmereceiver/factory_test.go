// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvmereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestArrayToSet(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected map[string]struct{}
	}{
		{
			name:     "empty array",
			input:    []string{},
			expected: map[string]struct{}{},
		},
		{
			name:  "single item",
			input: []string{"nvme0n1"},
			expected: map[string]struct{}{
				"nvme0n1": {},
			},
		},
		{
			name:  "multiple items",
			input: []string{"nvme0n1", "nvme1n1", "nvme2n1"},
			expected: map[string]struct{}{
				"nvme0n1": {},
				"nvme1n1": {},
				"nvme2n1": {},
			},
		},
		{
			name:  "duplicate items",
			input: []string{"nvme0n1", "nvme0n1", "nvme1n1"},
			expected: map[string]struct{}{
				"nvme0n1": {},
				"nvme1n1": {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := arrayToSet(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	config := createDefaultConfig().(*Config)
	assert.NotNil(t, config)
	assert.Empty(t, config.Resources)
}

func TestCreateMetricsReceiver(t *testing.T) {
	testCases := []struct {
		name      string
		resources []string
	}{
		{
			name:      "no resources",
			resources: []string{},
		},
		{
			name:      "with resources",
			resources: []string{"nvme0n1", "nvme1n1"},
		},
		{
			name:      "with wildcard",
			resources: []string{"*"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Resources = tc.resources

			receiver, err := createMetricsReceiver(
				context.Background(),
				receivertest.NewNopSettings(),
				cfg,
				consumertest.NewNop(),
			)

			require.NoError(t, err)
			require.NotNil(t, receiver)
		})
	}
}
