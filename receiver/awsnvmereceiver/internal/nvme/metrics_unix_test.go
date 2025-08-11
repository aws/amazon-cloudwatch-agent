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
		wantEBS *EBSMetrics
		wantIS  *InstanceStoreMetrics
		wantErr string
	}{
		{
			name: "valid EBS log page",
			input: func() []byte {
				metrics := EBSMetrics{
					EBSMagic:              ebsMagic,
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
					t.Fatalf("failed to create EBS test data: %v", err)
				}
				return buf.Bytes()
			}(),
			wantEBS: &EBSMetrics{
				EBSMagic:              ebsMagic,
				ReadOps:               100,
				WriteOps:              200,
				ReadBytes:             1024,
				WriteBytes:            2048,
				TotalReadTime:         5000,
				TotalWriteTime:        6000,
				EBSIOPSExceeded:       10,
				EBSThroughputExceeded: 20,
			},
		},
		{
			name: "invalid EBS magic number",
			input: func() []byte {
				metrics := EBSMetrics{
					EBSMagic: 0x12345678,
				}
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create invalid EBS test data: %v", err)
				}
				return buf.Bytes()
			}(),
			wantErr: ErrUnsupportedMagic.Error(),
		},
		{
			name: "valid Instance Store log page",
			input: func() []byte {
				metrics := InstanceStoreMetrics{
					Magic:                 instanceStoreMagic,
					ReadOps:               111,
					WriteOps:              222,
					ReadBytes:             333,
					WriteBytes:            444,
					TotalReadTime:         555,
					TotalWriteTime:        666,
					EC2IOPSExceeded:       777,
					EC2ThroughputExceeded: 888,
					QueueLength:           999,
				}
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create Instance Store test data: %v", err)
				}
				return buf.Bytes()
			}(),
			wantIS: &InstanceStoreMetrics{
				Magic:                 instanceStoreMagic,
				ReadOps:               111,
				WriteOps:              222,
				ReadBytes:             333,
				WriteBytes:            444,
				TotalReadTime:         555,
				TotalWriteTime:        666,
				EC2IOPSExceeded:       777,
				EC2ThroughputExceeded: 888,
				QueueLength:           999,
			},
		},
		{
			name: "invalid Instance Store magic number",
			input: func() []byte {
				metrics := InstanceStoreMetrics{
					Magic: 0x87654321,
				}
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create invalid Instance Store test data: %v", err)
				}
				return buf.Bytes()
			}(),
			wantErr: ErrUnsupportedMagic.Error(),
		},
		{
			name:    "empty data",
			input:   []byte{},
			wantErr: ErrParseLogPage.Error(),
		},
		{
			name: "unsupported magic",
			input: func() []byte {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, 0xDEADBEEFDEADBEEF)
				return buf
			}(),
			wantErr: ErrUnsupportedMagic.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLogPage(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			switch v := got.(type) {
			case EBSMetrics:
				if tt.wantEBS == nil {
					t.Fatalf("expected non-EBS result, got EBSMetrics: %+v", v)
				}
				if v != *tt.wantEBS {
					t.Errorf("EBSMetrics mismatch:\n got: %+v\nwant: %+v", v, *tt.wantEBS)
				}
			case InstanceStoreMetrics:
				if tt.wantIS == nil {
					t.Fatalf("expected non-InstanceStore result, got InstanceStoreMetrics: %+v", v)
				}
				if v != *tt.wantIS {
					t.Errorf("InstanceStoreMetrics mismatch:\n got: %+v\nwant: %+v", v, *tt.wantIS)
				}
			default:
				t.Fatalf("unexpected type: %T", v)
			}
		})
	}
}

func TestParseInstanceStoreMetrics(t *testing.T) {
	var expected InstanceStoreMetrics
	expected.Magic = 0x12345678
	expected.Reserved = 0x9ABCDEF0
	expected.ReadOps = 100
	expected.WriteOps = 200
	expected.ReadBytes = 300
	expected.WriteBytes = 400
	expected.TotalReadTime = 500
	expected.TotalWriteTime = 600
	expected.EC2IOPSExceeded = 700
	expected.EC2ThroughputExceeded = 800
	expected.QueueLength = 900
	expected.NumHistograms = 5
	expected.NumBins = 32
	expected.IOSizeRange = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}

	for i := uint64(0); i < 32; i++ {
		expected.Bounds[i].Lower = 1000 + i
		expected.Bounds[i].Upper = 2000 + i
	}

	for h := uint64(0); h < 5; h++ {
		for b := uint64(0); b < 32; b++ {
			expected.Histograms[h].Read[b] = h*1000 + b
			expected.Histograms[h].Write[b] = h*2000 + b
		}
	}

	// Encode struct into bytes
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, &expected); err != nil {
		t.Fatalf("failed to write binary: %v", err)
	}
	rawBytes := buf.Bytes()

	// Parse bytes into new struct
	var actual InstanceStoreMetrics
	if err := binary.Read(bytes.NewReader(rawBytes), binary.LittleEndian, &actual); err != nil {
		t.Fatalf("failed to read binary: %v", err)
	}

	if actual != expected {
		t.Fatalf("parsed struct does not match expected\nExpected: %+v\nActual: %+v", expected, actual)
	}
}
