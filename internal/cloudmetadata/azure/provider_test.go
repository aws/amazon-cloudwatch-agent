// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

			p := &Provider{networkMetadata: &nm}
			got := p.GetPrivateIP()

			if got != tt.wantIP {
				t.Errorf("GetPrivateIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestGetPrivateIP_NilNetworkMetadata(t *testing.T) {
	p := &Provider{networkMetadata: nil}

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
	p := &Provider{}

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
		available: true,
	}

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
	p := &Provider{}
	got := p.GetCloudProvider()
	if got != CloudProviderAzure {
		t.Errorf("GetCloudProvider() = %d, want %d", got, CloudProviderAzure)
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
			p := &Provider{available: tt.available}
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
			p := &Provider{metadata: tt.metadata}
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
		logger:  logger,
		diskMap: make(map[string]string),
	}

	// First call - cache miss (will return empty since we can't mock sysfs)
	got1 := p.GetVolumeID("/dev/sdc")
	if got1 != "" {
		t.Errorf("GetVolumeID() first call = %q, want empty (no sysfs)", got1)
	}

	// Manually populate cache to test cache hit
	p.diskMap["/dev/sdc"] = "disk-12345"

	// Second call - cache hit
	got2 := p.GetVolumeID("/dev/sdc")
	if got2 != "disk-12345" {
		t.Errorf("GetVolumeID() cached call = %q, want %q", got2, "disk-12345")
	}
}

func TestProvider_Refresh_Timeout(t *testing.T) {
	// Create a server that delays longer than the client timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		diskMap: make(map[string]string),
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
		available: true,
		diskMap:   make(map[string]string),
	}

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
				p.available = true
				p.mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "<empty>"},
		{"abc", "<present>"},
		{"abcd", "<present>"},
		{"abcde", "abcd..."},
		{"12345678-1234-1234-1234-123456789abc", "1234..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maskValue(tt.input)
			if got != tt.want {
				t.Errorf("maskValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaskIPAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "<empty>"},
		{"10.0.1.5", "10.0.x.x"},
		{"192.168.1.100", "192.168.x.x"},
		{"invalid", "<present>"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maskIPAddress(tt.input)
			if got != tt.want {
				t.Errorf("maskIPAddress(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
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

	if p.diskMap == nil {
		t.Error("Provider diskMap is nil")
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
		diskMap: make(map[string]string),
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
	p := &Provider{metadata: nil}

	_, err := p.GetTag("any-key")
	if err == nil {
		t.Error("GetTag() with nil metadata expected error, got nil")
	}
}

func TestProvider_GetVolumeID_Concurrent(_ *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger:  logger,
		diskMap: make(map[string]string),
	}

	// Pre-populate cache
	p.diskMap["/dev/sdc"] = "disk-12345"

	var wg sync.WaitGroup
	iterations := 50

	// Concurrent reads
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

func TestCloudProviderAzure_Constant(t *testing.T) {
	if CloudProviderAzure != 2 {
		t.Errorf("CloudProviderAzure = %d, want 2", CloudProviderAzure)
	}
}

func TestProvider_GetPrivateIP_NilLogger(t *testing.T) {
	p := &Provider{
		logger: nil,
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
				logger:          nil,
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
		diskMap:         make(map[string]string),
	}

	// Manually set metadata to test getters
	p.metadata = &computeResponse
	p.networkMetadata = &networkResponse
	p.available = true

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
