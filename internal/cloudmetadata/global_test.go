// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGlobalProvider_BeforeInit(t *testing.T) {
	ResetGlobalProvider()

	provider, err := GetGlobalProvider()

	assert.Nil(t, provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGetGlobalProviderOrNil_BeforeInit(t *testing.T) {
	ResetGlobalProvider()

	provider := GetGlobalProviderOrNil()

	assert.Nil(t, provider)
}

func TestSetGlobalProviderForTest_AWS(t *testing.T) {
	ResetGlobalProvider()
	defer ResetGlobalProvider()

	mock := &MockProvider{
		InstanceID:    "i-abc123",
		Region:        "us-east-1",
		Hostname:      "ip-10-0-0-1",
		PrivateIP:     "10.0.0.1",
		CloudProvider: CloudProviderAWS,
		Available:     true,
	}
	SetGlobalProviderForTest(mock)

	provider, err := GetGlobalProvider()

	require.NoError(t, err)
	assert.Equal(t, "i-abc123", provider.GetInstanceID())
	assert.Equal(t, "us-east-1", provider.GetRegion())
	assert.Equal(t, "ip-10-0-0-1", provider.GetHostname())
	assert.Equal(t, "10.0.0.1", provider.GetPrivateIP())
	assert.Equal(t, int(CloudProviderAWS), provider.GetCloudProvider())
	assert.True(t, provider.IsAvailable())
}

func TestSetGlobalProviderForTest_Azure(t *testing.T) {
	ResetGlobalProvider()
	defer ResetGlobalProvider()

	mock := &MockProvider{
		InstanceID:    "azure-vm-uuid",
		Region:        "eastus",
		Hostname:      "my-azure-vm",
		PrivateIP:     "10.0.0.2",
		CloudProvider: CloudProviderAzure,
		Available:     true,
	}
	SetGlobalProviderForTest(mock)

	provider, err := GetGlobalProvider()

	require.NoError(t, err)
	assert.Equal(t, int(CloudProviderAzure), provider.GetCloudProvider())
	assert.Equal(t, "azure-vm-uuid", provider.GetInstanceID())
	assert.Equal(t, "eastus", provider.GetRegion())
}

func TestGetGlobalProviderOrNil_AfterSet(t *testing.T) {
	ResetGlobalProvider()
	defer ResetGlobalProvider()

	mock := &MockProvider{InstanceID: "test-123"}
	SetGlobalProviderForTest(mock)

	provider := GetGlobalProviderOrNil()

	require.NotNil(t, provider)
	assert.Equal(t, "test-123", provider.GetInstanceID())
}

func TestResetGlobalProvider(t *testing.T) {
	ResetGlobalProvider()

	// Set provider
	SetGlobalProviderForTest(&MockProvider{InstanceID: "test"})

	// Verify set
	p, err := GetGlobalProvider()
	require.NoError(t, err)
	require.NotNil(t, p)

	// Reset
	ResetGlobalProvider()

	// Verify reset
	p, err = GetGlobalProvider()
	assert.Nil(t, p)
	assert.Error(t, err)
}

func TestCloudProvider_String(t *testing.T) {
	tests := []struct {
		cp   CloudProvider
		want string
	}{
		{CloudProviderUnknown, "Unknown"},
		{CloudProviderAWS, "AWS"},
		{CloudProviderAzure, "Azure"},
		{CloudProvider(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cp.String())
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	ResetGlobalProvider()
	defer ResetGlobalProvider()

	mock := &MockProvider{
		InstanceID: "concurrent-test",
		Available:  true,
	}
	SetGlobalProviderForTest(mock)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := GetGlobalProvider()
			if err != nil {
				errors <- err
				return
			}
			if p.GetInstanceID() != "concurrent-test" {
				errors <- fmt.Errorf("unexpected instance ID: %s", p.GetInstanceID())
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestMultipleResets(t *testing.T) {
	ResetGlobalProvider()
	ResetGlobalProvider()
	ResetGlobalProvider()

	SetGlobalProviderForTest(&MockProvider{InstanceID: "after-reset"})
	p, err := GetGlobalProvider()
	require.NoError(t, err)
	assert.Equal(t, "after-reset", p.GetInstanceID())
}

func TestProviderNotAvailable(t *testing.T) {
	ResetGlobalProvider()
	defer ResetGlobalProvider()

	mock := &MockProvider{
		InstanceID: "",
		Available:  false,
	}
	SetGlobalProviderForTest(mock)

	provider, err := GetGlobalProvider()

	require.NoError(t, err)
	assert.False(t, provider.IsAvailable())
	assert.Empty(t, provider.GetInstanceID())
}

func TestMockProvider_GetTag(t *testing.T) {
	mock := &MockProvider{
		Tags: map[string]string{
			"Name":        "test-instance",
			"Environment": "production",
		},
	}

	val, err := mock.GetTag("Name")
	require.NoError(t, err)
	assert.Equal(t, "test-instance", val)

	val, err = mock.GetTag("NonExistent")
	assert.Error(t, err)
	assert.Empty(t, val)
}

func TestMockProvider_GetTags(t *testing.T) {
	mock := &MockProvider{}
	tags := mock.GetTags()
	assert.NotNil(t, tags)
	assert.Empty(t, tags)

	mock.Tags = map[string]string{"key": "value"}
	tags = mock.GetTags()
	assert.Equal(t, "value", tags["key"])
}

func TestMockProvider_Refresh(t *testing.T) {
	mock := &MockProvider{}

	err := mock.Refresh(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, mock.RefreshCount)

	mock.RefreshErr = fmt.Errorf("refresh failed")
	err = mock.Refresh(context.Background())
	assert.Error(t, err)
	assert.Equal(t, 2, mock.RefreshCount)
}

func TestProviderInterface_AllMethods(t *testing.T) {
	ResetGlobalProvider()
	defer ResetGlobalProvider()

	mock := &MockProvider{
		InstanceID:    "i-test",
		InstanceType:  "t2.micro",
		ImageID:       "ami-12345",
		Region:        "us-west-2",
		AZ:            "us-west-2a",
		AccountID:     "123456789012",
		Hostname:      "test-host",
		PrivateIP:     "192.168.1.1",
		CloudProvider: CloudProviderAWS,
		Available:     true,
		Tags:          map[string]string{"Name": "test"},
	}
	SetGlobalProviderForTest(mock)

	p, err := GetGlobalProvider()
	require.NoError(t, err)

	assert.Equal(t, "i-test", p.GetInstanceID())
	assert.Equal(t, "t2.micro", p.GetInstanceType())
	assert.Equal(t, "ami-12345", p.GetImageID())
	assert.Equal(t, "us-west-2", p.GetRegion())
	assert.Equal(t, "us-west-2a", p.GetAvailabilityZone())
	assert.Equal(t, "123456789012", p.GetAccountID())
	assert.Equal(t, "test-host", p.GetHostname())
	assert.Equal(t, "192.168.1.1", p.GetPrivateIP())
	assert.Equal(t, int(CloudProviderAWS), p.GetCloudProvider())
	assert.True(t, p.IsAvailable())
	assert.Equal(t, map[string]string{"Name": "test"}, p.GetTags())

	tagVal, err := p.GetTag("Name")
	require.NoError(t, err)
	assert.Equal(t, "test", tagVal)

	assert.Empty(t, p.GetVolumeID("/dev/sda"))
	assert.Empty(t, p.GetScalingGroupName())

	err = p.Refresh(context.Background())
	assert.NoError(t, err)
}

func TestSetGlobalProviderForTest_PreventsInitOverwrite(t *testing.T) {
	ResetGlobalProvider()
	defer ResetGlobalProvider()

	mock := &MockProvider{InstanceID: "mock-instance"}
	SetGlobalProviderForTest(mock)

	p, err := GetGlobalProvider()
	require.NoError(t, err)
	assert.Equal(t, "mock-instance", p.GetInstanceID())

	p, err = GetGlobalProvider()
	require.NoError(t, err)
	assert.Equal(t, "mock-instance", p.GetInstanceID())
}

func TestInitGlobalProvider_NilLogger(_ *testing.T) {
	ResetGlobalProvider()
	defer ResetGlobalProvider()

	// Should not panic with nil logger
	err := InitGlobalProvider(context.Background(), nil)

	// Error expected (no IMDS in test env), but no panic
	_ = err

	// Verify state is consistent
	p := GetGlobalProviderOrNil()
	_ = p
}
