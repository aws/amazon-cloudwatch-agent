// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const (
	azureChassisAssetTag = "7783-7084-3265-9085-8269-3286-77"
	microsoftCorporation = "Microsoft Corporation"

	// Azure IMDS endpoints
	azureIMDSEndpoint        = "http://169.254.169.254/metadata/instance/compute"
	azureIMDSNetworkEndpoint = "http://169.254.169.254/metadata/instance/network"
	azureAPIVersion          = "2021-02-01"

	// Default refresh interval
	defaultRefreshInterval = 5 * time.Minute
)

// Configurable paths for testing and cross-platform support
var (
	// DMI paths - can be overridden for testing
	DMISysVendorPath    = "/sys/class/dmi/id/sys_vendor"
	DMIChassisAssetPath = "/sys/class/dmi/id/chassis_asset_tag"
	// Sysfs block device path
	SysBlockPath = "/sys/block"
)

// ComputeMetadata represents Azure IMDS compute metadata
type ComputeMetadata struct {
	Location          string                    `json:"location"`
	Name              string                    `json:"name"`
	VMID              string                    `json:"vmId"`
	VMSize            string                    `json:"vmSize"`
	SubscriptionID    string                    `json:"subscriptionId"`
	ResourceGroupName string                    `json:"resourceGroupName"`
	VMScaleSetName    string                    `json:"vmScaleSetName"`
	TagsList          []ComputeTagsListMetadata `json:"tagsList"`
	StorageProfile    StorageProfile            `json:"storageProfile"`
}

// StorageProfile represents Azure VM storage configuration
type StorageProfile struct {
	DataDisks []DataDisk `json:"dataDisks"`
}

// DataDisk represents an attached data disk
type DataDisk struct {
	Lun  string `json:"lun"`
	Name string `json:"name"`
}

// ComputeTagsListMetadata represents a tag in Azure IMDS
type ComputeTagsListMetadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NetworkMetadata represents Azure IMDS network response
type NetworkMetadata struct {
	Interface []NetworkInterface `json:"interface"`
}

// NetworkInterface represents a network interface in Azure IMDS
type NetworkInterface struct {
	IPv4 NetworkIPv4 `json:"ipv4"`
}

// NetworkIPv4 represents IPv4 configuration
type NetworkIPv4 struct {
	IPAddress []NetworkIPAddress `json:"ipAddress"`
}

// NetworkIPAddress represents an IP address entry
type NetworkIPAddress struct {
	PrivateIPAddress string `json:"privateIpAddress"`
	PublicIPAddress  string `json:"publicIpAddress"`
}

// Provider implements the metadata provider interface for Azure.
// Uses raw HTTP calls to Azure IMDS instead of an SDK because:
// 1. Azure does not provide an official Go SDK for IMDS access
// 2. The IMDS API is simple and stable (HTTP GET with Metadata header)
// 3. Avoids additional dependencies for a straightforward HTTP API
//
// Azure metadata is refreshed periodically (unlike AWS which caches at startup) because:
// 1. Azure VMs can have tags and network config updated dynamically
// 2. VM scale set membership can change
// 3. Azure IMDS is designed for periodic polling with fast response times
type Provider struct {
	logger     *zap.Logger
	httpClient *http.Client

	// Cached metadata
	mu              sync.RWMutex
	metadata        *ComputeMetadata
	networkMetadata *NetworkMetadata
	lastRefresh     time.Time
	refreshInterval time.Duration
	available       atomic.Bool // Use atomic for lock-free access

	// For testing: override IMDS endpoint
	imdsEndpoint string
}

// NewProvider creates a new Azure metadata provider
func NewProvider(ctx context.Context, logger *zap.Logger) (*Provider, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	p := &Provider{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		refreshInterval: defaultRefreshInterval,
	}

	// Initial fetch
	if err := p.Refresh(ctx); err != nil {
		logger.Warn("Failed to fetch initial Azure metadata", zap.Error(err))
		// Don't return error - allow agent to start even if metadata unavailable
	}

	return p, nil
}

// IsAzure detects if running on Azure using multiple methods:
// IsAzure detects if running on Azure.
// Detection order:
// 1. DMI sys_vendor check
// 2. DMI chassis asset tag check (Azure-specific)
// 3. IMDS probe as fallback (for containers without DMI access)
func IsAzure() bool {
	// 1. Check sys_vendor for Microsoft
	if data, err := os.ReadFile(DMISysVendorPath); err == nil {
		if strings.Contains(strings.TrimSpace(string(data)), microsoftCorporation) {
			return true
		}
	}

	// 3. Check chassis asset tag (Azure-specific identifier)
	if data, err := os.ReadFile(DMIChassisAssetPath); err == nil {
		if strings.TrimSpace(string(data)) == azureChassisAssetTag {
			return true
		}
	}

	// 3. IMDS probe fallback (for containers without DMI)
	if probeAzureIMDS() {
		return true
	}

	return false
}

// probeAzureIMDS attempts a quick IMDS request to detect Azure.
// Uses short timeout to avoid blocking on non-Azure environments.
func probeAzureIMDS() bool {
	client := &http.Client{Timeout: 1 * time.Second}
	req, err := http.NewRequest("GET", azureIMDSEndpoint+"?api-version="+azureAPIVersion, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Metadata", "true")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Azure IMDS returns 200 with JSON body
	return resp.StatusCode == http.StatusOK
}

// GetInstanceID returns the Azure VM ID
func (p *Provider) GetInstanceID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}
	return p.metadata.VMID
}

// GetInstanceType returns the Azure VM size
func (p *Provider) GetInstanceType() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}
	return p.metadata.VMSize
}

// GetImageID returns a composite image identifier
// Azure doesn't have a single image ID like AWS AMI
// We return the VM ID as identifier
func (p *Provider) GetImageID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}
	return p.metadata.VMID
}

// GetRegion returns the Azure location
func (p *Provider) GetRegion() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}
	return p.metadata.Location
}

// GetAvailabilityZone returns the Azure zone
func (p *Provider) GetAvailabilityZone() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Azure zones are not always available in IMDS
	// Return empty string for now
	return ""
}

// GetAccountID returns the Azure subscription ID
func (p *Provider) GetAccountID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}
	return p.metadata.SubscriptionID
}

// GetTags returns all Azure tags as a map
func (p *Provider) GetTags() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return make(map[string]string)
	}

	tags := make(map[string]string)
	for _, tag := range p.metadata.TagsList {
		tags[tag.Name] = tag.Value
	}
	return tags
}

// GetTag returns a specific tag value
func (p *Provider) GetTag(key string) (string, error) {
	tags := p.GetTags()
	if val, ok := tags[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("tag %s not found", key)
}

// GetVolumeID returns the disk name for a given device.
// Maps Linux device (e.g., /dev/sdc) to Azure disk name using LUN from sysfs.
// The Logical Unit Number (LUN) is extracted from the device symlink: /sys/block/sdc/device -> ../../../0:0:0:LUN
// https://learn.microsoft.com/en-us/azure/virtual-machines/linux/azure-to-guest-disk-mapping
func (p *Provider) GetVolumeID(deviceName string) string {
	// Extract device name (e.g., "sdc" from "/dev/sdc")
	devName := strings.TrimPrefix(deviceName, "/dev/")

	// Get LUN from sysfs symlink
	lun := p.getLUNFromDevice(devName)
	if lun == "" {
		return ""
	}

	// Match LUN to disk name from IMDS metadata
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}

	for _, disk := range p.metadata.StorageProfile.DataDisks {
		if disk.Lun == lun {
			return disk.Name
		}
	}

	return ""
}

// getLUNFromDevice extracts LUN from sysfs device symlink.
// Example: /sys/block/sda/device -> ../../../0:0:0:0 returns "0"
func (p *Provider) getLUNFromDevice(devName string) string {
	devicePath := filepath.Join(SysBlockPath, devName, "device")

	target, err := os.Readlink(devicePath)
	if err != nil {
		return ""
	}

	// Target format: ../../../H:C:T:L (Host:Channel:Target:LUN)
	parts := strings.Split(target, ":")
	if len(parts) < 4 {
		return ""
	}

	return parts[len(parts)-1]
}

// GetScalingGroupName returns the VM Scale Set name
func (p *Provider) GetScalingGroupName() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}
	return p.metadata.VMScaleSetName
}

// GetResourceGroupName returns the Azure resource group name
func (p *Provider) GetResourceGroupName() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}
	return p.metadata.ResourceGroupName
}

// Refresh fetches the latest metadata from Azure IMDS
func (p *Provider) Refresh(ctx context.Context) error {
	startTime := time.Now()

	endpoint := azureIMDSEndpoint
	if p.imdsEndpoint != "" {
		endpoint = p.imdsEndpoint
	}

	p.logger.Debug("[cloudmetadata/azure] Fetching compute metadata from IMDS...",
		zap.String("endpoint", endpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Metadata", "true")
	q := req.URL.Query()
	q.Add("format", "json")
	q.Add("api-version", azureAPIVersion)
	req.URL.RawQuery = q.Encode()

	resp, err := p.httpClient.Do(req)
	duration := time.Since(startTime)
	if err != nil {
		p.available.Store(false)
		p.logger.Warn("[cloudmetadata/azure] IMDS request failed",
			zap.Error(err),
			zap.Duration("duration", duration))
		return fmt.Errorf("failed to query Azure IMDS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.available.Store(false)
		p.logger.Warn("[cloudmetadata/azure] IMDS returned non-200 status",
			zap.Int("status", resp.StatusCode),
			zap.Duration("duration", duration))
		return fmt.Errorf("Azure IMDS replied with status code: %s", resp.Status)
	}

	p.logger.Debug("[cloudmetadata/azure] IMDS response received",
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", duration))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Azure IMDS reply: %w", err)
	}

	var metadata ComputeMetadata
	if err := json.Unmarshal(respBody, &metadata); err != nil {
		return fmt.Errorf("failed to decode Azure IMDS reply: %w", err)
	}

	p.mu.Lock()
	p.metadata = &metadata
	p.lastRefresh = time.Now()
	p.available.Store(true)
	p.mu.Unlock()

	p.logger.Debug("[cloudmetadata/azure] Parsed compute metadata",
		zap.String("vmSize", metadata.VMSize),
		zap.String("location", metadata.Location),
		zap.String("resourceGroup", metadata.ResourceGroupName))

	// Fetch network metadata (non-fatal if it fails)
	if err := p.refreshNetwork(ctx); err != nil {
		p.logger.Debug("[cloudmetadata/azure] Failed to fetch network metadata (non-fatal)",
			zap.Error(err))
	}

	return nil
}

// refreshNetwork fetches network metadata from Azure IMDS
// Called after compute metadata fetch; failure is non-fatal
func (p *Provider) refreshNetwork(ctx context.Context) error {
	startTime := time.Now()
	p.logger.Debug("[cloudmetadata/azure] Refreshing network metadata...",
		zap.String("endpoint", azureIMDSNetworkEndpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, azureIMDSNetworkEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create network request: %w", err)
	}

	req.Header.Add("Metadata", "true")
	q := req.URL.Query()
	q.Add("format", "json")
	q.Add("api-version", azureAPIVersion)
	req.URL.RawQuery = q.Encode()

	resp, err := p.httpClient.Do(req)
	duration := time.Since(startTime)
	if err != nil {
		p.logger.Debug("[cloudmetadata/azure] Network IMDS request failed",
			zap.Error(err),
			zap.Duration("duration", duration))
		return fmt.Errorf("failed to query Azure IMDS network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.logger.Debug("[cloudmetadata/azure] Network IMDS returned non-200 status",
			zap.Int("status", resp.StatusCode),
			zap.Duration("duration", duration))
		return fmt.Errorf("Azure IMDS network replied with status code: %s", resp.Status)
	}

	p.logger.Debug("[cloudmetadata/azure] Network IMDS response received",
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", duration))

	var networkMetadata NetworkMetadata
	if err := json.NewDecoder(resp.Body).Decode(&networkMetadata); err != nil {
		return fmt.Errorf("failed to decode network metadata: %w", err)
	}

	p.mu.Lock()
	p.networkMetadata = &networkMetadata
	p.mu.Unlock()

	privateIP := ""
	if len(networkMetadata.Interface) > 0 && len(networkMetadata.Interface[0].IPv4.IPAddress) > 0 {
		privateIP = networkMetadata.Interface[0].IPv4.IPAddress[0].PrivateIPAddress
	}

	if privateIP != "" {
		p.logger.Debug("[cloudmetadata/azure] Network metadata refreshed")
	} else {
		p.logger.Debug("[cloudmetadata/azure] Network metadata refreshed but no private IP found")
	}

	return nil
}

// IsAvailable returns true if metadata has been successfully fetched
func (p *Provider) IsAvailable() bool {
	return p.available.Load()
}

// GetHostname returns the Azure VM name
func (p *Provider) GetHostname() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.metadata == nil {
		return ""
	}
	return p.metadata.Name
}

// GetPrivateIP returns the Azure VM private IP address
func (p *Provider) GetPrivateIP() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.networkMetadata == nil {
		p.logger.Debug("[cloudmetadata/azure] GetPrivateIP called: network metadata not available")
		return ""
	}
	if len(p.networkMetadata.Interface) == 0 {
		p.logger.Debug("[cloudmetadata/azure] GetPrivateIP called: no network interfaces found")
		return ""
	}
	if len(p.networkMetadata.Interface[0].IPv4.IPAddress) == 0 {
		p.logger.Debug("[cloudmetadata/azure] GetPrivateIP called: no IP addresses found")
		return ""
	}

	return p.networkMetadata.Interface[0].IPv4.IPAddress[0].PrivateIPAddress
}

// GetCloudProvider returns the cloud provider type.
// Returns 2 (CloudProviderAzure from internal/cloudmetadata/constants.go).
// NOTE: Cannot import cloudmetadata package here due to import cycle.
func (p *Provider) GetCloudProvider() int {
	return 2 // Must match cloudmetadata.CloudProviderAzure
}
