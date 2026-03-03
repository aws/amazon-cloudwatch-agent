// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azure

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestFetchStorageProfile_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := &Provider{client: server.Client(), endpoint: server.URL}
	_, err := p.DeviceToSerialMap(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IMDS returned 500")
}
