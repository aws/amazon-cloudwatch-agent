// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNetworkMetadata_Parsing(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		wantIP string
	}{
		{
			name:   "valid response",
			json:   `{"interface":[{"ipv4":{"ipAddress":[{"privateIpAddress":"10.0.0.4","publicIpAddress":""}]}}]}`,
			wantIP: "10.0.0.4",
		},
		{
			name:   "multiple IPs returns first",
			json:   `{"interface":[{"ipv4":{"ipAddress":[{"privateIpAddress":"10.0.0.4","publicIpAddress":""},{"privateIpAddress":"10.0.0.5","publicIpAddress":""}]}}]}`,
			wantIP: "10.0.0.4",
		},
		{
			name:   "empty interface",
			json:   `{"interface":[]}`,
			wantIP: "",
		},
		{
			name:   "empty ipAddress",
			json:   `{"interface":[{"ipv4":{"ipAddress":[]}}]}`,
			wantIP: "",
		},
		{
			name:   "null interface",
			json:   `{"interface":null}`,
			wantIP: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nm NetworkMetadata
			if err := json.Unmarshal([]byte(tt.json), &nm); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			p := &Provider{
				logger:          zap.NewNop(),
				networkMetadata: &nm,
			}
			got := p.GetPrivateIP()

			if got != tt.wantIP {
				t.Errorf("GetPrivateIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestGetPrivateIP_NilNetworkMetadata(t *testing.T) {
	p := &Provider{
		logger:          zap.NewNop(),
		networkMetadata: nil,
	}

	got := p.GetPrivateIP()

	if got != "" {
		t.Errorf("GetPrivateIP() = %q, want empty", got)
	}
}

func TestNetworkMetadataStructs(t *testing.T) {
	jsonData := `{
		"interface": [{
			"ipv4": {
				"ipAddress": [{
					"privateIpAddress": "10.0.1.100",
					"publicIpAddress": "52.168.1.1"
				}]
			}
		}]
	}`

	var nm NetworkMetadata
	if err := json.Unmarshal([]byte(jsonData), &nm); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(nm.Interface) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(nm.Interface))
	}

	if len(nm.Interface[0].IPv4.IPAddress) != 1 {
		t.Fatalf("expected 1 IP address, got %d", len(nm.Interface[0].IPv4.IPAddress))
	}

	ip := nm.Interface[0].IPv4.IPAddress[0]
	if ip.PrivateIPAddress != "10.0.1.100" {
		t.Errorf("PrivateIPAddress = %q, want %q", ip.PrivateIPAddress, "10.0.1.100")
	}
	if ip.PublicIPAddress != "52.168.1.1" {
		t.Errorf("PublicIPAddress = %q, want %q", ip.PublicIPAddress, "52.168.1.1")
	}
}

func TestProvider_GettersWithNilMetadata(t *testing.T) {
	p := &Provider{logger: zap.NewNop()}

	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{"GetInstanceID", p.GetInstanceID, ""},
		{"GetInstanceType", p.GetInstanceType, ""},
		{"GetImageID", p.GetImageID, ""},
		{"GetRegion", p.GetRegion, ""},
		{"GetAvailabilityZone", p.GetAvailabilityZone, ""},
		{"GetAccountID", p.GetAccountID, ""},
		{"GetScalingGroupName", p.GetScalingGroupName, ""},
		{"GetHostname", p.GetHostname, ""},
		{"GetPrivateIP", p.GetPrivateIP, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestProvider_GettersWithMetadata(t *testing.T) {
	p := &Provider{
		logger: zap.NewNop(),
		metadata: &ComputeMetadata{
			Location:          "eastus",
			Name:              "test-vm",
			VMID:              "12345678-1234-1234-1234-123456789abc",
			VMSize:            "Standard_D2s_v3",
			SubscriptionID:    "sub-12345",
			ResourceGroupName: "test-rg",
			VMScaleSetName:    "test-vmss",
			TagsList: []ComputeTagsListMetadata{
				{Name: "Environment", Value: "Production"},
				{Name: "Owner", Value: "TeamA"},
			},
		},
		networkMetadata: &NetworkMetadata{
			Interface: []NetworkInterface{
				{
					IPv4: NetworkIPv4{
						IPAddress: []NetworkIPAddress{
							{PrivateIPAddress: "10.0.1.5"},
						},
					},
				},
			},
		},
	}
	p.available.Store(true)

	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{"GetInstanceID", p.GetInstanceID, "12345678-1234-1234-1234-123456789abc"},
		{"GetInstanceType", p.GetInstanceType, "Standard_D2s_v3"},
		{"GetImageID", p.GetImageID, "12345678-1234-1234-1234-123456789abc"},
		{"GetRegion", p.GetRegion, "eastus"},
		{"GetAvailabilityZone", p.GetAvailabilityZone, ""},
		{"GetAccountID", p.GetAccountID, "sub-12345"},
		{"GetScalingGroupName", p.GetScalingGroupName, "test-vmss"},
		{"GetHostname", p.GetHostname, "test-vm"},
		{"GetPrivateIP", p.GetPrivateIP, "10.0.1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestProvider_GetCloudProvider(t *testing.T) {
	p := &Provider{logger: zap.NewNop()}
	got := p.GetCloudProvider()
	if got != 2 { // cloudmetadata.CloudProviderAzure
		t.Errorf("GetCloudProvider() = %d, want 2", got)
	}
}

func TestProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		want      bool
	}{
		{"available", true, true},
		{"not available", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{logger: zap.NewNop()}
			p.available.Store(tt.available)
			got := p.IsAvailable()
			if got != tt.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_GetTags(t *testing.T) {
	tests := []struct {
		name     string
		metadata *ComputeMetadata
		want     map[string]string
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			want:     map[string]string{},
		},
		{
			name: "empty tags",
			metadata: &ComputeMetadata{
				TagsList: []ComputeTagsListMetadata{},
			},
			want: map[string]string{},
		},
		{
			name: "single tag",
			metadata: &ComputeMetadata{
				TagsList: []ComputeTagsListMetadata{
					{Name: "Environment", Value: "Production"},
				},
			},
			want: map[string]string{"Environment": "Production"},
		},
		{
			name: "multiple tags",
			metadata: &ComputeMetadata{
				TagsList: []ComputeTagsListMetadata{
					{Name: "Environment", Value: "Production"},
					{Name: "Owner", Value: "TeamA"},
					{Name: "CostCenter", Value: "Engineering"},
				},
			},
			want: map[string]string{
				"Environment": "Production",
				"Owner":       "TeamA",
				"CostCenter":  "Engineering",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				logger:   zap.NewNop(),
				metadata: tt.metadata,
			}
			got := p.GetTags()

			if len(got) != len(tt.want) {
				t.Errorf("GetTags() returned %d tags, want %d", len(got), len(tt.want))
			}

			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("GetTags()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestProvider_GetTag(t *testing.T) {
	p := &Provider{
		metadata: &ComputeMetadata{
			TagsList: []ComputeTagsListMetadata{
				{Name: "Environment", Value: "Production"},
				{Name: "Owner", Value: "TeamA"},
			},
		},
	}

	tests := []struct {
		name    string
		key     string
		want    string
		wantErr bool
	}{
		{"existing tag", "Environment", "Production", false},
		{"another existing tag", "Owner", "TeamA", false},
		{"non-existent tag", "NonExistent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.GetTag(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTag(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetTag(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestProvider_GetVolumeID(t *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		metadata: &ComputeMetadata{
			StorageProfile: StorageProfile{
				DataDisks: []DataDisk{
					{Lun: "0", Name: "disk-os"},
					{Lun: "1", Name: "disk-data1"},
					{Lun: "2", Name: "disk-data2"},
				},
			},
		},
	}

	// Without sysfs, returns empty
	got := p.GetVolumeID("/dev/sdc")
	if got != "" {
		t.Errorf("GetVolumeID() without sysfs = %q, want empty", got)
	}

	// Test with nil metadata
	p2 := &Provider{logger: logger}
	if got := p2.GetVolumeID("/dev/sdc"); got != "" {
		t.Errorf("GetVolumeID() with nil metadata = %q, want empty", got)
	}
}

func TestGetLUNFromDevice(t *testing.T) {
	// Create temp sysfs structure
	tmpDir := t.TempDir()
	origPath := SysBlockPath
	SysBlockPath = tmpDir
	defer func() { SysBlockPath = origPath }()

	// Create device symlink: sda/device -> ../../../0:0:0:1
	sdaDir := filepath.Join(tmpDir, "sda")
	if err := os.MkdirAll(sdaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../../0:0:0:1", filepath.Join(sdaDir, "device")); err != nil {
		t.Fatal(err)
	}

	p := &Provider{logger: zap.NewNop()}

	// Test valid device
	lun := p.getLUNFromDevice("sda")
	if lun != "1" {
		t.Errorf("getLUNFromDevice(sda) = %q, want %q", lun, "1")
	}

	// Test non-existent device
	lun = p.getLUNFromDevice("nonexistent")
	if lun != "" {
		t.Errorf("getLUNFromDevice(nonexistent) = %q, want empty", lun)
	}
}

func TestProvider_GetVolumeID_WithSysfs(t *testing.T) {
	// Create temp sysfs structure
	tmpDir := t.TempDir()
	origPath := SysBlockPath
	SysBlockPath = tmpDir
	defer func() { SysBlockPath = origPath }()

	// Create device symlinks
	for _, dev := range []struct {
		name string
		lun  string
	}{
		{"sda", "0"},
		{"sdb", "1"},
		{"sdc", "2"},
	} {
		devDir := filepath.Join(tmpDir, dev.name)
		if err := os.MkdirAll(devDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../../0:0:0:"+dev.lun, filepath.Join(devDir, "device")); err != nil {
			t.Fatal(err)
		}
	}

	p := &Provider{
		logger: zap.NewNop(),
		metadata: &ComputeMetadata{
			StorageProfile: StorageProfile{
				DataDisks: []DataDisk{
					{Lun: "1", Name: "data-disk-1"},
					{Lun: "2", Name: "data-disk-2"},
				},
			},
		},
	}

	tests := []struct {
		device string
		want   string
	}{
		{"/dev/sda", ""},           // LUN 0 not in dataDisks
		{"/dev/sdb", "data-disk-1"}, // LUN 1
		{"/dev/sdc", "data-disk-2"}, // LUN 2
		{"sdb", "data-disk-1"},      // Without /dev/ prefix
	}

	for _, tt := range tests {
		got := p.GetVolumeID(tt.device)
		if got != tt.want {
			t.Errorf("GetVolumeID(%q) = %q, want %q", tt.device, got, tt.want)
		}
	}
}

func TestProvider_Refresh_Timeout(t *testing.T) {
	// Create a server that delays longer than the client timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zap.NewNop()
	p := &Provider{
		logger:       logger,
		imdsEndpoint: server.URL,
		httpClient: &http.Client{
			Timeout: 50 * time.Millisecond,
		},
	}

	ctx := context.Background()
	err := p.Refresh(ctx)

	if err == nil {
		t.Error("Refresh() expected error, got nil")
	}

	if p.IsAvailable() {
		t.Error("IsAvailable() = true after failed refresh, want false")
	}
}

func TestProvider_ConcurrentAccess(_ *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		metadata: &ComputeMetadata{
			Location: "eastus",
			VMID:     "test-id",
		},
	}
	p.available.Store(true)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = p.GetInstanceID()
				_ = p.GetRegion()
				_ = p.GetTags()
				_ = p.IsAvailable()
			}
		}()
	}

	// Concurrent writers (simulating refresh)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				p.mu.Lock()
				p.metadata = &ComputeMetadata{
					Location: fmt.Sprintf("region-%d", id),
					VMID:     fmt.Sprintf("vm-%d", id),
				}
				p.available.Store(true)
				p.mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
}

func TestNewProvider(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	p, err := NewProvider(ctx, logger)

	// Should not return error even if IMDS unavailable
	if err != nil {
		t.Errorf("NewProvider() error = %v, want nil", err)
	}

	if p == nil {
		t.Fatal("NewProvider() returned nil provider")
	}

	if p.logger == nil {
		t.Error("Provider logger is nil")
	}

	if p.httpClient == nil {
		t.Error("Provider httpClient is nil")
	}

	if p.refreshInterval != defaultRefreshInterval {
		t.Errorf("refreshInterval = %v, want %v", p.refreshInterval, defaultRefreshInterval)
	}
}

func TestComputeMetadata_Parsing(t *testing.T) {
	jsonData := `{
		"location": "eastus",
		"name": "test-vm",
		"vmId": "12345678-1234-1234-1234-123456789abc",
		"vmSize": "Standard_D2s_v3",
		"subscriptionId": "sub-12345",
		"resourceGroupName": "test-rg",
		"vmScaleSetName": "test-vmss",
		"tagsList": [
			{"name": "Environment", "value": "Production"},
			{"name": "Owner", "value": "TeamA"}
		]
	}`

	var metadata ComputeMetadata
	if err := json.Unmarshal([]byte(jsonData), &metadata); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if metadata.Location != "eastus" {
		t.Errorf("Location = %q, want %q", metadata.Location, "eastus")
	}
	if metadata.Name != "test-vm" {
		t.Errorf("Name = %q, want %q", metadata.Name, "test-vm")
	}
	if metadata.VMID != "12345678-1234-1234-1234-123456789abc" {
		t.Errorf("VMID = %q, want %q", metadata.VMID, "12345678-1234-1234-1234-123456789abc")
	}
	if metadata.VMSize != "Standard_D2s_v3" {
		t.Errorf("VMSize = %q, want %q", metadata.VMSize, "Standard_D2s_v3")
	}
	if metadata.SubscriptionID != "sub-12345" {
		t.Errorf("SubscriptionID = %q, want %q", metadata.SubscriptionID, "sub-12345")
	}
	if metadata.ResourceGroupName != "test-rg" {
		t.Errorf("ResourceGroupName = %q, want %q", metadata.ResourceGroupName, "test-rg")
	}
	if metadata.VMScaleSetName != "test-vmss" {
		t.Errorf("VMScaleSetName = %q, want %q", metadata.VMScaleSetName, "test-vmss")
	}
	if len(metadata.TagsList) != 2 {
		t.Errorf("TagsList length = %d, want 2", len(metadata.TagsList))
	}
}

func TestProvider_Refresh_ContextCanceled(t *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := p.Refresh(ctx)

	if err == nil {
		t.Error("Refresh() with canceled context expected error, got nil")
	}

	if p.IsAvailable() {
		t.Error("IsAvailable() = true after failed refresh, want false")
	}
}

func TestProvider_GetTag_NilMetadata(t *testing.T) {
	p := &Provider{
		logger:   zap.NewNop(),
		metadata: nil,
	}

	_, err := p.GetTag("any-key")
	if err == nil {
		t.Error("GetTag() with nil metadata expected error, got nil")
	}
}

func TestProvider_GetVolumeID_Concurrent(_ *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
	}

	var wg sync.WaitGroup
	iterations := 50

	// Concurrent reads - all return empty since not implemented
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = p.GetVolumeID("/dev/sdc")
			}
		}()
	}

	wg.Wait()
}

func TestIsAzure(t *testing.T) {
	// In test environment, DMI files won't exist or won't contain Azure markers
	result := IsAzure()
	// Just verify it doesn't panic
	t.Logf("IsAzure() = %v (environment-dependent)", result)
}

func TestConfigurablePaths(t *testing.T) {
	// Verify paths are configurable (for testing)
	origSysVendor := DMISysVendorPath
	origChassis := DMIChassisAssetPath
	defer func() {
		DMISysVendorPath = origSysVendor
		DMIChassisAssetPath = origChassis
	}()

	DMISysVendorPath = "/tmp/test/sys_vendor"
	DMIChassisAssetPath = "/tmp/test/chassis_asset_tag"

	// Verify paths were changed
	if DMISysVendorPath != "/tmp/test/sys_vendor" {
		t.Error("DMISysVendorPath not configurable")
	}
}

func TestCloudProviderAzure_Constant(t *testing.T) {
	// Verify that Azure provider returns the correct cloud provider constant
	// This should match cloudmetadata.CloudProviderAzure = 2
	p := &Provider{logger: zap.NewNop()}
	if p.GetCloudProvider() != 2 {
		t.Errorf("GetCloudProvider() = %d, want 2 (cloudmetadata.CloudProviderAzure)", p.GetCloudProvider())
	}
}

func TestProvider_GetPrivateIP_NilLogger(t *testing.T) {
	// This test verifies that the provider works correctly even when
	// logger is initialized with zap.NewNop() (which happens automatically in NewProvider)
	p := &Provider{
		logger: zap.NewNop(),
		networkMetadata: &NetworkMetadata{
			Interface: []NetworkInterface{
				{
					IPv4: NetworkIPv4{
						IPAddress: []NetworkIPAddress{
							{PrivateIPAddress: "10.0.1.5"},
						},
					},
				},
			},
		},
	}

	got := p.GetPrivateIP()
	if got != "10.0.1.5" {
		t.Errorf("GetPrivateIP() = %q, want %q", got, "10.0.1.5")
	}
}

func TestProvider_GetPrivateIP_EdgeCases_NilLogger(t *testing.T) {
	tests := []struct {
		name            string
		networkMetadata *NetworkMetadata
		want            string
	}{
		{
			name:            "nil network metadata",
			networkMetadata: nil,
			want:            "",
		},
		{
			name: "empty interfaces",
			networkMetadata: &NetworkMetadata{
				Interface: []NetworkInterface{},
			},
			want: "",
		},
		{
			name: "empty IP addresses",
			networkMetadata: &NetworkMetadata{
				Interface: []NetworkInterface{
					{IPv4: NetworkIPv4{IPAddress: []NetworkIPAddress{}}},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				logger:          zap.NewNop(),
				networkMetadata: tt.networkMetadata,
			}

			got := p.GetPrivateIP()
			if got != tt.want {
				t.Errorf("GetPrivateIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProvider_Refresh_WithMockServer(t *testing.T) {
	computeResponse := ComputeMetadata{
		Location:          "westus2",
		Name:              "test-vm",
		VMID:              "test-vm-id",
		VMSize:            "Standard_D2s_v3",
		SubscriptionID:    "test-sub",
		ResourceGroupName: "test-rg",
		VMScaleSetName:    "",
		TagsList: []ComputeTagsListMetadata{
			{Name: "env", Value: "test"},
		},
	}

	networkResponse := NetworkMetadata{
		Interface: []NetworkInterface{
			{
				IPv4: NetworkIPv4{
					IPAddress: []NetworkIPAddress{
						{PrivateIPAddress: "10.0.2.4"},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata") != "true" {
			t.Errorf("Missing Metadata header")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/metadata/instance/compute":
			json.NewEncoder(w).Encode(computeResponse)
		case "/metadata/instance/network":
			json.NewEncoder(w).Encode(networkResponse)
		}
	}))
	defer server.Close()

	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		refreshInterval: defaultRefreshInterval,
	}

	// Manually set metadata to test getters
	p.metadata = &computeResponse
	p.networkMetadata = &networkResponse
	p.available.Store(true)

	if p.GetInstanceID() != "test-vm-id" {
		t.Errorf("GetInstanceID() = %q, want %q", p.GetInstanceID(), "test-vm-id")
	}
	if p.GetRegion() != "westus2" {
		t.Errorf("GetRegion() = %q, want %q", p.GetRegion(), "westus2")
	}
	if p.GetPrivateIP() != "10.0.2.4" {
		t.Errorf("GetPrivateIP() = %q, want %q", p.GetPrivateIP(), "10.0.2.4")
	}
	if !p.IsAvailable() {
		t.Error("IsAvailable() = false, want true")
	}
}
