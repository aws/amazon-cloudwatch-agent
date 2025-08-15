// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ebs

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestParseRawData(t *testing.T) {
	scraper := NewScraper()

	tests := []struct {
		name    string
		input   []byte
		want    *Metrics
		wantErr string
	}{
		{
			name: "valid EBS log page",
			input: func() []byte {
				metrics := Metrics{
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
			want: &Metrics{
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
				metrics := Metrics{
					EBSMagic: 0x12345678,
				}
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create invalid EBS test data: %v", err)
				}
				return buf.Bytes()
			}(),
			wantErr: errInvalidEbsMagic.Error(),
		},
		{
			name:    "input too short",
			input:   []byte{0x01, 0x02},
			wantErr: errInvalidEbsMagic.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInterface, err := scraper.ParseRawData(tt.input)
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
			got, ok := gotInterface.(Metrics)
			if !ok {
				t.Fatalf("expected Metrics type but got %T", gotInterface)
			}

			if got != *tt.want {
				t.Errorf("metrics mismatch:\n got: %+v\nwant: %+v", got, *tt.want)
			}
		})
	}
}
