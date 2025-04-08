// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// The following code is based on https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/pkg/metrics/nvme.go

// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package nvme

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestParseLogPage(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    EBSMetrics
		wantErr string
	}{
		{
			name: "valid log page",
			input: func() []byte {
				metrics := EBSMetrics{
					EBSMagic:              0x3C23B510,
					ReadOps:               100,
					WriteOps:              200,
					ReadBytes:             1024,
					WriteBytes:            2048,
					TotalReadTime:         5000,
					TotalWriteTime:        6000,
					EBSIOPSExceeded:       10,
					EBSThroughputExceeded: 20,
				}
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create test data: %v", err)
				}
				return buf.Bytes()
			}(),
			want: EBSMetrics{
				EBSMagic:              0x3C23B510,
				ReadOps:               100,
				WriteOps:              200,
				ReadBytes:             1024,
				WriteBytes:            2048,
				TotalReadTime:         5000,
				TotalWriteTime:        6000,
				EBSIOPSExceeded:       10,
				EBSThroughputExceeded: 20,
			},
			wantErr: "",
		},
		{
			name: "invalid magic number",
			input: func() []byte {
				metrics := EBSMetrics{
					EBSMagic: 0x12345678,
				}
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create test data: %v", err)
				}
				return buf.Bytes()
			}(),
			want:    EBSMetrics{},
			wantErr: ErrInvalidEBSMagic.Error(),
		},
		{
			name:    "empty data",
			input:   []byte{},
			want:    EBSMetrics{},
			wantErr: ErrParseLogPage.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLogPage(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("parseLogPage() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("parseLogPage() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				return
			}

			if err != nil {
				t.Errorf("parseLogPage() unexpected error = %v", err)
				return
			}

			if got.EBSMagic != tt.want.EBSMagic {
				t.Errorf("parseLogPage() magic number = %x, want %x", got.EBSMagic, tt.want.EBSMagic)
			}
			if got.ReadOps != tt.want.ReadOps {
				t.Errorf("parseLogPage() ReadOps = %v, want %v", got.ReadOps, tt.want.ReadOps)
			}
			if got.WriteOps != tt.want.WriteOps {
				t.Errorf("parseLogPage() WriteOps = %v, want %v", got.WriteOps, tt.want.WriteOps)
			}
			if got.ReadBytes != tt.want.ReadBytes {
				t.Errorf("parseLogPage() ReadBytes = %v, want %v", got.ReadBytes, tt.want.ReadBytes)
			}
			if got.WriteBytes != tt.want.WriteBytes {
				t.Errorf("parseLogPage() WriteBytes = %v, want %v", got.WriteBytes, tt.want.WriteBytes)
			}
		})
	}
}
