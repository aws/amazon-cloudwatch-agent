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
	InstanceID       string
	InstanceType     string
	ImageID          string
	Region           string
	AZ               string
	AccountID        string
	Hostname         string
	PrivateIP        string
	CloudProvider    CloudProvider
	Available        bool
	Tags             map[string]string
	ResourceGroup    string // For Azure mocking
	ScalingGroupName string // For ASG (AWS) or VMSS (Azure)
	RefreshErr       error
	RefreshCount     int
}

func (m *MockProvider) GetInstanceID() string       { return m.InstanceID }
func (m *MockProvider) GetInstanceType() string     { return m.InstanceType }
func (m *MockProvider) GetImageID() string          { return m.ImageID }
func (m *MockProvider) GetRegion() string           { return m.Region }
func (m *MockProvider) GetAvailabilityZone() string { return m.AZ }
func (m *MockProvider) GetAccountID() string        { return m.AccountID }
func (m *MockProvider) GetHostname() string         { return m.Hostname }
func (m *MockProvider) GetPrivateIP() string        { return m.PrivateIP }
func (m *MockProvider) GetCloudProvider() int       { return int(m.CloudProvider) }
func (m *MockProvider) IsAvailable() bool           { return m.Available }

func (m *MockProvider) GetTags() map[string]string {
	if m.Tags == nil {
		return make(map[string]string)
	}
	// Return a copy to prevent external mutation
	tagsCopy := make(map[string]string, len(m.Tags))
	for k, v := range m.Tags {
		tagsCopy[k] = v
	}
	return tagsCopy
}

func (m *MockProvider) GetTag(key string) (string, error) {
	if m.Tags == nil {
		return "", fmt.Errorf("tag not found: %s", key)
	}
	if v, ok := m.Tags[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("tag not found: %s", key)
}

func (m *MockProvider) GetVolumeID(_ string) string  { return "" }
func (m *MockProvider) GetScalingGroupName() string  { return m.ScalingGroupName }
func (m *MockProvider) GetResourceGroupName() string { return m.ResourceGroup }
func (m *MockProvider) Refresh(_ context.Context) error {
	m.RefreshCount++
	return m.RefreshErr
}
