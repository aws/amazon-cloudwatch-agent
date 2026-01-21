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
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CloudProviderAzure is the constant for Azure cloud provider (matches cloudmetadata.CloudProviderAzure)
const CloudProviderAzure = 2

const (
	// DMI paths for Azure detection
	dmiSysVendorPath     = "/sys/class/dmi/id/sys_vendor"
	dmiChassisAssetPath  = "/sys/class/dmi/id/chassis_asset_tag"
	azureChassisAssetTag = "7783-7084-3265-9085-8269-3286-77"
	microsoftCorporation = "Microsoft Corporation"

	// Azure IMDS endpoints
	azureIMDSEndpoint        = "http://169.254.169.254/metadata/instance/compute"
	azureIMDSNetworkEndpoint = "http://169.254.169.254/metadata/instance/network"
	azureAPIVersion          = "2021-02-01"

	// Default refresh interval
	defaultRefreshInterval = 5 * time.Minute
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

// Provider implements the metadata provider interface for Azure
type Provider struct {
	logger     *zap.Logger
	httpClient *http.Client

	// Cached metadata
	mu              sync.RWMutex
	metadata        *ComputeMetadata
	networkMetadata *NetworkMetadata
	lastRefresh     time.Time
	refreshInterval time.Duration
	available       bool

	// Disk mapping cache
	diskMap map[string]string // device name -> disk ID
}

// NewProvider creates a new Azure metadata provider
func NewProvider(ctx context.Context, logger *zap.Logger) (*Provider, error) {
	p := &Provider{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		refreshInterval: defaultRefreshInterval,
		diskMap:         make(map[string]string),
	}

	// Initial fetch
	if err := p.Refresh(ctx); err != nil {
		logger.Warn("Failed to fetch initial Azure metadata", zap.Error(err))
		// Don't return error - allow agent to start even if metadata unavailable
	}

	return p, nil
}

// IsAzure detects if running on Azure by checking DMI information
func IsAzure() bool {
	// Check sys_vendor
	if data, err := os.ReadFile(dmiSysVendorPath); err == nil {
		if strings.Contains(strings.TrimSpace(string(data)), microsoftCorporation) {
			return true
		}
	}

	// Check chassis asset tag (Azure-specific)
	if data, err := os.ReadFile(dmiChassisAssetPath); err == nil {
		if strings.TrimSpace(string(data)) == azureChassisAssetTag {
			return true
		}
	}

	return false
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

// GetVolumeID returns the disk ID for a given device name
// Uses LUN-based mapping between Linux device names and Azure managed disks
func (p *Provider) GetVolumeID(deviceName string) string {
	// Check cache first with read lock
	p.mu.RLock()
	if diskID, ok := p.diskMap[deviceName]; ok {
		p.mu.RUnlock()
		return diskID
	}
	p.mu.RUnlock()

	// Cache miss - compute disk ID
	diskID := p.mapDeviceToDisk(deviceName)
	if diskID != "" {
		// Store in cache with write lock
		p.mu.Lock()
		p.diskMap[deviceName] = diskID
		p.mu.Unlock()
	}

	return diskID
}

// mapDeviceToDisk maps a Linux device name to an Azure disk ID using LUN
func (p *Provider) mapDeviceToDisk(deviceName string) string {
	// Extract device name (e.g., "sdc" from "/dev/sdc")
	devName := strings.TrimPrefix(deviceName, "/dev/")

	// Get LUN from sysfs
	lun, err := p.getLUNFromDevice(devName)
	if err != nil {
		p.logger.Debug("Failed to get LUN for device",
			zap.String("device", deviceName),
			zap.Error(err))
		return ""
	}

	p.logger.Debug("Device LUN mapping",
		zap.String("device", deviceName),
		zap.Int("lun", lun))

	return ""
}

// getLUNFromDevice reads the LUN number from sysfs for a given device
func (p *Provider) getLUNFromDevice(devName string) (int, error) {
	// Pattern: /sys/block/<device>/device/scsi_device/*/device/lun
	pattern := filepath.Join("/sys/block", devName, "device/scsi_device/*/device/lun")

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return -1, fmt.Errorf("no LUN file found for device %s", devName)
	}

	// Read the first match
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return -1, fmt.Errorf("failed to read LUN file: %w", err)
	}

	lun, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1, fmt.Errorf("failed to parse LUN: %w", err)
	}

	return lun, nil
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
	p.logger.Debug("[cloudmetadata/azure] Fetching compute metadata from IMDS...",
		zap.String("endpoint", azureIMDSEndpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, azureIMDSEndpoint, nil)
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
		p.mu.Lock()
		p.available = false
		p.mu.Unlock()
		p.logger.Warn("[cloudmetadata/azure] IMDS request failed",
			zap.Error(err),
			zap.Duration("duration", duration))
		return fmt.Errorf("failed to query Azure IMDS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.mu.Lock()
		p.available = false
		p.mu.Unlock()
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
	p.available = true
	// Clear disk cache on refresh to pick up new disks
	p.diskMap = make(map[string]string)
	p.mu.Unlock()

	p.logger.Debug("[cloudmetadata/azure] Parsed compute metadata",
		zap.String("vmId", maskValue(metadata.VMID)),
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
		p.logger.Debug("[cloudmetadata/azure] Network metadata refreshed",
			zap.String("privateIP", maskIPAddress(privateIP)))
	} else {
		p.logger.Debug("[cloudmetadata/azure] Network metadata refreshed but no private IP found")
	}

	return nil
}

// maskValue masks sensitive values for logging
func maskValue(value string) string {
	if value == "" {
		return "<empty>"
	}
	if len(value) <= 4 {
		return "<present>"
	}
	return value[:4] + "..."
}

// maskIPAddress masks IP addresses for logging (e.g., 10.0.x.x)
func maskIPAddress(ip string) string {
	if ip == "" {
		return "<empty>"
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".x.x"
	}
	return "<present>"
}

// IsAvailable returns true if metadata has been successfully fetched
func (p *Provider) IsAvailable() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.available
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
		if p.logger != nil {
			p.logger.Debug("[cloudmetadata/azure] GetPrivateIP called: network metadata not available")
		}
		return ""
	}
	if len(p.networkMetadata.Interface) == 0 {
		if p.logger != nil {
			p.logger.Debug("[cloudmetadata/azure] GetPrivateIP called: no network interfaces found")
		}
		return ""
	}
	if len(p.networkMetadata.Interface[0].IPv4.IPAddress) == 0 {
		if p.logger != nil {
			p.logger.Debug("[cloudmetadata/azure] GetPrivateIP called: no IP addresses found")
		}
		return ""
	}

	privateIP := p.networkMetadata.Interface[0].IPv4.IPAddress[0].PrivateIPAddress
	if p.logger != nil {
		p.logger.Debug("[cloudmetadata/azure] GetPrivateIP called",
			zap.String("value", maskIPAddress(privateIP)))
	}
	return privateIP
}

// GetCloudProvider returns the cloud provider type (Azure = 2)
func (p *Provider) GetCloudProvider() int {
	return CloudProviderAzure
}
