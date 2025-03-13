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
			wantErr:        false,
		},
		{
			name:           "Partial format nvme0n",
			device:         "nvme0n",
			wantController: 0,
			wantNamespace:  -1,
			wantPartition:  -1,
			wantErr:        false,
		},
		{
			name:           "Invalid format nvme0np",
			device:         "nvme0np",
			wantController: 0,
			wantNamespace:  -1,
			wantPartition:  -1,
			wantErr:        false,
		},
		{
			name:           "Invalid format nvme0n1p",
			device:         "nvme0n1p",
			wantController: 0,
			wantNamespace:  1,
			wantPartition:  -1,
			wantErr:        false,
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
			wantErr:        false,
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

func TestSubstring(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		left      int
		right     int
		want      string
	}{
		{
			name: "Valid substring",
			s:    "hello",
			left: 1,
			right: 4,
			want: "ell",
		},
		{
			name: "Left boundary negative",
			s:    "hello",
			left: -1,
			right: 4,
			want: "",
		},
		{
			name: "Right boundary exceeds length",
			s:    "hello",
			left: 1,
			right: 10,
			want: "",
		},
		{
			name: "Left greater than right",
			s:    "hello",
			left: 4,
			right: 2,
			want: "",
		},
		{
			name: "Empty string",
			s:    "",
			left: 0,
			right: 1,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := substring(tt.s, tt.left, tt.right); got != tt.want {
				t.Errorf("substring() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAtoiOrNegative(t *testing.T) {
	tests := []struct {
		name string
		input string
		want int
	}{
		{
			name: "Valid number",
			input: "123",
			want: 123,
		},
		{
			name: "Empty string",
			input: "",
			want: -1,
		},
		{
			name: "Invalid number",
			input: "abc",
			want: -1,
		},
		{
			name: "Zero",
			input: "0",
			want: 0,
		},
		{
			name: "Negative number",
			input: "-123",
			want: -123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := atoiOrNegative(tt.input); got != tt.want {
				t.Errorf("atoiOrNegative() = %v, want %v", got, tt.want)
			}
		})
	}
}
