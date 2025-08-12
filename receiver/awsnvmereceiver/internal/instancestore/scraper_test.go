package instancestore

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// TestParseRawData tests parsing raw bytes into InstanceStoreMetrics struct
func TestParseRawData(t *testing.T) {
	scraper := NewScraper()

	tests := []struct {
		name    string
		input   []byte
		want    *InstanceStoreMetrics
		wantErr string
	}{
		{
			name: "valid Instance Store log page",
			input: func() []byte {
				metrics := InstanceStoreMetrics{
					Magic:                 InstanceStoreMagic,
					Reserved:              0xABCD1234,
					ReadOps:               111,
					WriteOps:              222,
					ReadBytes:             333,
					WriteBytes:            444,
					TotalReadTime:         555,
					TotalWriteTime:        666,
					EBSIOPSExceeded:       0, // Not applicable
					EBSThroughputExceeded: 0, // Not applicable
					EC2IOPSExceeded:       777,
					EC2ThroughputExceeded: 888,
					QueueLength:           999,
					NumHistograms:         5,
					NumBins:               32,
					IOSizeRange:           [8]uint32{1, 2, 3, 4, 5, 6, 7, 8},
				}

				for i := 0; i < 32; i++ {
					metrics.Bounds[i].Lower = uint64(1000 + i)
					metrics.Bounds[i].Upper = uint64(2000 + i)
				}

				for h := 0; h < 5; h++ {
					for b := 0; b < 32; b++ {
						metrics.Histograms[h].Read[b] = uint64(h*1000 + b)
						metrics.Histograms[h].Write[b] = uint64(h*2000 + b)
					}
				}

				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create Instance Store test data: %v", err)
				}
				return buf.Bytes()
			}(),
			want: &InstanceStoreMetrics{
				Magic:                 InstanceStoreMagic,
				Reserved:              0xABCD1234,
				ReadOps:               111,
				WriteOps:              222,
				ReadBytes:             333,
				WriteBytes:            444,
				TotalReadTime:         555,
				TotalWriteTime:        666,
				EBSIOPSExceeded:       0,
				EBSThroughputExceeded: 0,
				EC2IOPSExceeded:       777,
				EC2ThroughputExceeded: 888,
				QueueLength:           999,
				NumHistograms:         5,
				NumBins:               32,
				IOSizeRange:           [8]uint32{1, 2, 3, 4, 5, 6, 7, 8},
			},
		},
		{
			name: "invalid magic number",
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
			wantErr: ErrInvalidInstanceStoreMagic.Error(),
		},
		{
			name:    "input too short",
			input:   []byte{0x01, 0x02},
			wantErr: ErrInvalidInstanceStoreMagic.Error(),
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
			got, ok := gotInterface.(InstanceStoreMetrics)
			if !ok {
				t.Fatalf("expected InstanceStoreMetrics type but got %T", gotInterface)
			}

			// Only compare the fields that were set in the 'want' struct above (partial equality)
			if got.Magic != tt.want.Magic ||
				got.Reserved != tt.want.Reserved ||
				got.ReadOps != tt.want.ReadOps ||
				got.WriteOps != tt.want.WriteOps ||
				got.ReadBytes != tt.want.ReadBytes ||
				got.WriteBytes != tt.want.WriteBytes ||
				got.TotalReadTime != tt.want.TotalReadTime ||
				got.TotalWriteTime != tt.want.TotalWriteTime ||
				got.EBSIOPSExceeded != tt.want.EBSIOPSExceeded ||
				got.EBSThroughputExceeded != tt.want.EBSThroughputExceeded ||
				got.EC2IOPSExceeded != tt.want.EC2IOPSExceeded ||
				got.EC2ThroughputExceeded != tt.want.EC2ThroughputExceeded ||
				got.QueueLength != tt.want.QueueLength ||
				got.NumHistograms != tt.want.NumHistograms ||
				got.NumBins != tt.want.NumBins {
				t.Errorf("metrics mismatch:\n got: %+v\nwant: %+v", got, *tt.want)
			}
		})
	}
}

// TestBinaryEncodeDecode tests that encoding InstanceStoreMetrics to binary and decoding it back yields the same struct
func TestBinaryEncodeDecode(t *testing.T) {
	expected := InstanceStoreMetrics{
		Magic:                 InstanceStoreMagic,
		Reserved:              0x9ABCDEF0,
		ReadOps:               100,
		WriteOps:              200,
		ReadBytes:             300,
		WriteBytes:            400,
		TotalReadTime:         500,
		TotalWriteTime:        600,
		EBSIOPSExceeded:       0,
		EBSThroughputExceeded: 0,
		EC2IOPSExceeded:       700,
		EC2ThroughputExceeded: 800,
		QueueLength:           900,
		NumHistograms:         5,
		NumBins:               32,
		IOSizeRange:           [8]uint32{1, 2, 3, 4, 5, 6, 7, 8},
	}

	for i := 0; i < 32; i++ {
		expected.Bounds[i].Lower = uint64(1000 + i)
		expected.Bounds[i].Upper = uint64(2000 + i)
	}

	for h := 0; h < 5; h++ {
		for b := 0; b < 32; b++ {
			expected.Histograms[h].Read[b] = uint64(h*1000 + b)
			expected.Histograms[h].Write[b] = uint64(h*2000 + b)
		}
	}

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, &expected); err != nil {
		t.Fatalf("failed to write binary: %v", err)
	}
	rawBytes := buf.Bytes()

	var actual InstanceStoreMetrics
	if err := binary.Read(bytes.NewReader(rawBytes), binary.LittleEndian, &actual); err != nil {
		t.Fatalf("failed to read binary: %v", err)
	}

	// Check all fields for equality (can use reflect.DeepEqual if you want)
	if actual != expected {
		t.Fatalf("parsed struct does not match expected\nExpected: %+v\nActual: %+v", expected, actual)
	}
}
