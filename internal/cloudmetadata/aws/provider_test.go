// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockMetadataProvider is a mock implementation of ec2metadataprovider.MetadataProvider
type MockMetadataProvider struct {
	mock.Mock
}

func (m *MockMetadataProvider) Get(ctx context.Context) (imds.InstanceIdentityDocument, error) {
	args := m.Called(ctx)
	return args.Get(0).(imds.InstanceIdentityDocument), args.Error(1)
}

func (m *MockMetadataProvider) Hostname(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) InstanceID(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) InstanceTags(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockMetadataProvider) ClientIAMRole(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockMetadataProvider) InstanceTagValue(ctx context.Context, tagKey string) (string, error) {
	args := m.Called(ctx, tagKey)
	return args.String(0), args.Error(1)
}

func TestProvider_GetMetadata(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockMetadata := &MockMetadataProvider{}

	// Setup mock expectations
	expectedDoc := imds.InstanceIdentityDocument{
		InstanceID:       "i-1234567890abcdef0",
		InstanceType:     "t3.micro",
		ImageID:          "ami-0abcdef1234567890",
		Region:           "us-west-2",
		AvailabilityZone: "us-west-2a",
		AccountID:        "123456789012",
		PrivateIP:        "10.0.1.100",
	}

	mockMetadata.On("Get", ctx).Return(expectedDoc, nil)
	mockMetadata.On("Hostname", ctx).Return("ip-10-0-1-100.us-west-2.compute.internal", nil)

	// Create provider with mock
	provider := &Provider{
		logger:   logger,
		metadata: mockMetadata,
	}

	// Fetch metadata
	err := provider.fetchMetadata(ctx)
	assert.NoError(t, err)

	// Verify all fields are populated correctly
	assert.Equal(t, "i-1234567890abcdef0", provider.GetInstanceID())
	assert.Equal(t, "t3.micro", provider.GetInstanceType())
	assert.Equal(t, "ami-0abcdef1234567890", provider.GetImageID())
	assert.Equal(t, "us-west-2", provider.GetRegion())
	assert.Equal(t, "us-west-2a", provider.GetAvailabilityZone())
	assert.Equal(t, "123456789012", provider.GetAccountID())
	assert.Equal(t, "10.0.1.100", provider.GetPrivateIP())
	assert.Equal(t, "ip-10-0-1-100.us-west-2.compute.internal", provider.GetHostname())
	assert.True(t, provider.IsAvailable())
	assert.Equal(t, 1, provider.GetCloudProvider())

	mockMetadata.AssertExpectations(t)
}

func TestProvider_GetMetadata_Failure(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockMetadata := &MockMetadataProvider{}

	// Setup mock to return error
	mockMetadata.On("Get", ctx).Return(imds.InstanceIdentityDocument{}, assert.AnError)

	// Create provider with mock
	provider := &Provider{
		logger:   logger,
		metadata: mockMetadata,
	}

	// Fetch metadata should fail
	err := provider.fetchMetadata(ctx)
	assert.Error(t, err)
	assert.False(t, provider.IsAvailable())

	mockMetadata.AssertExpectations(t)
}

func TestProvider_Refresh(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockMetadata := &MockMetadataProvider{}

	// Setup initial metadata
	initialDoc := imds.InstanceIdentityDocument{
		InstanceID: "i-initial",
		Region:     "us-east-1",
	}

	// Setup updated metadata
	updatedDoc := imds.InstanceIdentityDocument{
		InstanceID: "i-updated",
		Region:     "us-west-2",
	}

	mockMetadata.On("Get", ctx).Return(initialDoc, nil).Once()
	mockMetadata.On("Hostname", ctx).Return("host1", nil).Once()
	mockMetadata.On("Get", ctx).Return(updatedDoc, nil).Once()
	mockMetadata.On("Hostname", ctx).Return("host2", nil).Once()

	provider := &Provider{
		logger:   logger,
		metadata: mockMetadata,
	}

	// Initial fetch
	err := provider.fetchMetadata(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "i-initial", provider.GetInstanceID())
	assert.Equal(t, "us-east-1", provider.GetRegion())

	// Refresh
	err = provider.Refresh(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "i-updated", provider.GetInstanceID())
	assert.Equal(t, "us-west-2", provider.GetRegion())

	mockMetadata.AssertExpectations(t)
}

func TestProvider_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockMetadata := &MockMetadataProvider{}

	doc := imds.InstanceIdentityDocument{
		InstanceID: "i-concurrent",
		Region:     "us-west-2",
	}

	mockMetadata.On("Get", ctx).Return(doc, nil)
	mockMetadata.On("Hostname", ctx).Return("hostname", nil)

	provider := &Provider{
		logger:   logger,
		metadata: mockMetadata,
	}

	err := provider.fetchMetadata(ctx)
	assert.NoError(t, err)

	// Test concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			assert.Equal(t, "i-concurrent", provider.GetInstanceID())
			assert.Equal(t, "us-west-2", provider.GetRegion())
			assert.True(t, provider.IsAvailable())
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	mockMetadata.AssertExpectations(t)
}
