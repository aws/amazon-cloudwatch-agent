//go:build linux

package nvme

import (
	"testing"
)

// Test GetRawData error path: opening file fails
func TestGetRawData_OpenFail(t *testing.T) {
	_, err := GetRawData("/non/existent/device/path")
	if err == nil {
		t.Errorf("expected error opening non-existent device path")
	}
}
