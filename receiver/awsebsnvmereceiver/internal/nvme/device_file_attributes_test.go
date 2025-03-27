// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"testing"
)

func TestParseNvmeDeviceFileName(t *testing.T) {
	tests := []struct {
		name           string
		device         string
		wantController int
		wantNamespace  int
		wantPartition  int
		wantErr        bool
	}{
		{
			name:           "Valid controller only",
			device:         "nvme0",
			wantController: 0,
			wantNamespace:  -1,
			wantPartition:  -1,
			wantErr:        false,
		},
		{
			name:           "Valid controller and namespace",
			device:         "nvme0n1",
			wantController: 0,
			wantNamespace:  1,
			wantPartition:  -1,
			wantErr:        false,
		},
		{
			name:           "Valid controller, namespace and partition",
			device:         "nvme0n1p2",
			wantController: 0,
			wantNamespace:  1,
			wantPartition:  2,
			wantErr:        false,
		},
		{
			name:    "Invalid prefix",
			device:  "abcd",
			wantErr: true,
		},
		{
			name:           "Invalid format nvmeanbp",
			device:         "nvmeanbp",
			wantController: -1,
			wantNamespace:  -1,
			wantPartition:  -1,
			wantErr:        true,
		},
		{
			name:           "Multiple digit controller",
			device:         "nvme12n1p2",
			wantController: 12,
			wantNamespace:  1,
			wantPartition:  2,
			wantErr:        false,
		},
		{
			name:           "Multiple digit namespace",
			device:         "nvme0n123",
			wantController: 0,
			wantNamespace:  123,
			wantPartition:  -1,
			wantErr:        false,
		},
		{
			name:           "Non-numeric controller",
			device:         "nvmean1p2",
			wantController: -1,
			wantNamespace:  1,
			wantPartition:  2,
			wantErr:        true,
		},
		{
			name:           "Wrong order",
			device:         "nvmep1n1",
			wantController: -1,
			wantNamespace:  -1,
			wantPartition:  -1,
			wantErr:        true,
		},
		{
			name:    "Empty string",
			device:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNvmeDeviceFileName(tt.device)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNvmeDeviceFileName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Controller() != tt.wantController {
					t.Errorf("Controller() = %v, want %v", got.Controller(), tt.wantController)
				}
				if got.Namespace() != tt.wantNamespace {
					t.Errorf("Namespace() = %v, want %v", got.Namespace(), tt.wantNamespace)
				}
				if got.Partition() != tt.wantPartition {
					t.Errorf("Partition() = %v, want %v", got.Partition(), tt.wantPartition)
				}
			}
		})
	}
}
