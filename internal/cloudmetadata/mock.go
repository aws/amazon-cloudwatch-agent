// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
	"fmt"
)

// MockProvider implements Provider interface for testing.
// This is exported so other packages can use it in their tests.
type MockProvider struct {
	InstanceID_       string
	InstanceType_     string
	ImageID_          string
	Region_           string
	AZ_               string
	AccountID_        string
	Hostname_         string
	PrivateIP_        string
	CloudProvider_    CloudProvider
	Available_        bool
	Tags_             map[string]string
	ScalingGroupName_ string
	RefreshErr        error
	RefreshCount      int
}

func (m *MockProvider) GetInstanceID() string       { return m.InstanceID_ }
func (m *MockProvider) GetInstanceType() string     { return m.InstanceType_ }
func (m *MockProvider) GetImageID() string          { return m.ImageID_ }
func (m *MockProvider) GetRegion() string           { return m.Region_ }
func (m *MockProvider) GetAvailabilityZone() string { return m.AZ_ }
func (m *MockProvider) GetAccountID() string        { return m.AccountID_ }
func (m *MockProvider) GetHostname() string         { return m.Hostname_ }
func (m *MockProvider) GetPrivateIP() string        { return m.PrivateIP_ }
func (m *MockProvider) GetCloudProvider() int       { return int(m.CloudProvider_) }
func (m *MockProvider) IsAvailable() bool           { return m.Available_ }

func (m *MockProvider) GetTags() map[string]string {
	if m.Tags_ == nil {
		return make(map[string]string)
	}
	return m.Tags_
}

func (m *MockProvider) GetTag(key string) (string, error) {
	if m.Tags_ == nil {
		return "", fmt.Errorf("tag not found: %s", key)
	}
	if v, ok := m.Tags_[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("tag not found: %s", key)
}

func (m *MockProvider) GetVolumeID(device string) string { return "" }
func (m *MockProvider) GetScalingGroupName() string      { return m.ScalingGroupName_ }
func (m *MockProvider) Refresh(ctx context.Context) error {
	m.RefreshCount++
	return m.RefreshErr
}
