// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseDevice(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sda", "sda"},
		{"sda1", "sda"},
		{"sda15", "sda"},
		{"sdb", "sdb"},
		{"sdb1", "sdb"},
		{"nvme0n1", "nvme0n1"},
		{"nvme0n1p1", "nvme0n1"},
		{"nvme0n1p15", "nvme0n1"},
		{"nvme1n1", "nvme1n1"},
		{"nvme1n1p2", "nvme1n1"},
		{"xvda", "xvda"},
		{"xvda1", "xvda"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, baseDevice(tt.input))
		})
	}
}
