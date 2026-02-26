// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
)

type stubProvider struct {
	region        string
	cloud         cloudprovider.CloudProvider
}

func (s *stubProvider) Region() string                             { return s.region }
func (s *stubProvider) InstanceID() string                         { return "i-test" }
func (s *stubProvider) Hostname() string                           { return "" }
func (s *stubProvider) InstanceType() string                       { return "" }
func (s *stubProvider) ImageID() string                            { return "" }
func (s *stubProvider) AccountID() string                          { return "" }
func (s *stubProvider) PrivateIP() string                          { return "" }
func (s *stubProvider) CloudProvider() cloudprovider.CloudProvider { return s.cloud }

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)
	assert.Equal(t, "disktagger", f.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, 5*time.Minute, cfg.RefreshInterval)
	assert.Equal(t, "device", cfg.DiskDeviceTagKey)
}

func TestCreateDiskProvider_NoCloud(t *testing.T) {
	cloudmetadata.SetForTest(nil)
	defer cloudmetadata.ResetForTest()

	set := processortest.NewNopSettings(component.MustNewType(typeStr))
	p := createDiskProvider(t.Context(), set)
	assert.Nil(t, p)
}

func TestCreateDiskProvider_UnsupportedCloud(t *testing.T) {
	cloudmetadata.SetForTest(&stubProvider{region: "somewhere", cloud: cloudprovider.Unknown})
	defer cloudmetadata.ResetForTest()

	set := processortest.NewNopSettings(component.MustNewType(typeStr))
	p := createDiskProvider(t.Context(), set)
	assert.Nil(t, p)
}

func TestCreateDiskProvider_Azure(t *testing.T) {
	cloudmetadata.SetForTest(&stubProvider{region: "eastus", cloud: cloudprovider.Azure})
	defer cloudmetadata.ResetForTest()

	set := processortest.NewNopSettings(component.MustNewType(typeStr))
	p := createDiskProvider(t.Context(), set)
	require.NotNil(t, p)
	// Azure provider returns a mapProvider
	_, ok := p.(*mapProvider)
	assert.True(t, ok)
}
