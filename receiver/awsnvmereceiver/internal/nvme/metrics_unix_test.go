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
	"errors"
	"os"
	"strings"
	"testing"
)

func TestGetRawData_OpenFileError(t *testing.T) {
	devicePath := "/non/existent/device"
	data, err := GetRawData(devicePath)
	if data != nil {
		t.Error("expected nil data")
	}
	if err == nil {
		t.Error("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist wrapped in error, got %v", err)
	}
	expectedMsg := "getNVMEMetrics: error opening device"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error message containing %q, got %v", expectedMsg, err)
	}
}

func TestGetRawData_ReadLogError(t *testing.T) {
	// Create a temporary file which is not an NVMe device
	tempFile, err := os.CreateTemp("", "non_nvme_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	data, err := GetRawData(tempFile.Name())
	if data != nil {
		t.Error("expected nil data")
	}
	if err == nil {
		t.Error("expected error")
	}
	expectedMsg := "getNVMEMetrics: error reading log page"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error message containing %q, got %v", expectedMsg, err)
	}
}
