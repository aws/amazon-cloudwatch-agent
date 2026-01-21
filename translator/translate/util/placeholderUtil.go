// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/tagutil"
)

const (
	instanceIdPlaceholder    = "{instance_id}"
	hostnamePlaceholder      = "{hostname}"
	localHostnamePlaceholder = "{local_hostname}" //regardless of ec2 metadata
	ipAddressPlaceholder     = "{ip_address}"
	awsRegionPlaceholder     = "{aws_region}"
	datePlaceholder          = "{date}"
	accountIdPlaceholder     = "{account_id}"

	unknownInstanceID   = "i-UNKNOWN"
	unknownHostname     = "UNKNOWN-HOST"
	unknownIPAddress    = "UNKNOWN-IP"
	unknownAwsRegion    = "UNKNOWN-REGION"
	unknownAccountID    = "UNKNOWN-ACCOUNT"
	unknownInstanceType = "UNKNOWN-TYPE"
	unknownImageID      = "UNKNOWN-AMI"

	awsPlaceholderPrefix   = "${aws:"
	azurePlaceholderPrefix = "${azure:"
)

type Metadata struct {
	InstanceID   string
	Hostname     string
	PrivateIP    string
	AccountID    string
	InstanceType string
	ImageID      string
}

type MetadataInfoProvider func() *Metadata

var ec2MetadataInfoProviderFunc = ec2MetadataInfoProvider

var Ec2MetadataInfoProvider = func() *Metadata {
	return ec2MetadataInfoProviderFunc()
}

func ec2MetadataInfoProvider() *Metadata {
	ec2 := ec2util.GetEC2UtilSingleton()
	return &Metadata{
		InstanceID:   ec2.InstanceID,
		Hostname:     ec2.Hostname,
		PrivateIP:    ec2.PrivateIP,
		AccountID:    ec2.AccountID,
		InstanceType: ec2.InstanceType,
		ImageID:      ec2.ImageID,
	}
}

func ResolvePlaceholder(placeholder string, metadata map[string]string) string {
	tmpString := placeholder
	if tmpString == "" {
		tmpString = instanceIdPlaceholder
	}
	for k, v := range metadata {
		tmpString = strings.Replace(tmpString, k, v, -1)
	}
	tmpString = strings.Replace(tmpString, datePlaceholder, time.Now().Format("2006-01-02"), -1)
	return tmpString
}

func defaultIfEmpty(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func GetMetadataInfo(provider MetadataInfoProvider) map[string]string {
	localHostname := getHostName()

	// Try cloudmetadata singleton first (supports multi-cloud)
	if cloudProvider := cloudmetadata.GetGlobalProviderOrNil(); cloudProvider != nil {
		cloudType := "Unknown"
		switch cloudProvider.GetCloudProvider() {
		case 1:
			cloudType = "AWS"
		case 2:
			cloudType = "Azure"
		}
		log.Printf("I! [placeholderUtil] Using cloudmetadata provider (cloud=%s)", cloudType)

		instanceID := defaultIfEmpty(cloudProvider.GetInstanceID(), unknownInstanceID)
		hostname := defaultIfEmpty(cloudProvider.GetHostname(), localHostname)
		privateIP := cloudProvider.GetPrivateIP()
		if privateIP == "" {
			log.Printf("D! [placeholderUtil] cloudmetadata returned empty PrivateIP, using local IP fallback")
			privateIP = getIpAddress()
		}
		region := defaultIfEmpty(cloudProvider.GetRegion(), unknownAwsRegion)
		accountID := defaultIfEmpty(cloudProvider.GetAccountID(), unknownAccountID)

		// Use agent config region if available (user override)
		if agent.Global_Config.Region != "" {
			region = agent.Global_Config.Region
		}

		log.Printf("I! [placeholderUtil] Resolved via cloudmetadata: instanceId=%s, hostname=%s, region=%s, accountId=%s, privateIP=%s",
			maskValue(instanceID), hostname, region, maskValue(accountID), maskIPAddress(privateIP))

		return map[string]string{
			instanceIdPlaceholder:    instanceID,
			hostnamePlaceholder:      hostname,
			localHostnamePlaceholder: localHostname,
			ipAddressPlaceholder:     privateIP,
			awsRegionPlaceholder:     region,
			accountIdPlaceholder:     accountID,
		}
	}

	// Fallback: Check if we're on Azure (legacy path)
	if isAzure() {
		log.Printf("D! [placeholderUtil] cloudmetadata not available, using legacy Azure provider")
		return getAzureMetadataInfo()
	}

	// Fallback: AWS legacy path using provider function
	log.Printf("D! [placeholderUtil] cloudmetadata not available, using legacy AWS provider")
	md := provider()

	instanceID := defaultIfEmpty(md.InstanceID, unknownInstanceID)
	hostname := defaultIfEmpty(md.Hostname, localHostname)
	ipAddress := defaultIfEmpty(md.PrivateIP, getIpAddress())
	awsRegion := defaultIfEmpty(agent.Global_Config.Region, unknownAwsRegion)
	accountID := defaultIfEmpty(md.AccountID, unknownAccountID)

	log.Printf("D! [placeholderUtil] Resolved via legacy: instanceId=%s, region=%s, privateIP=%s",
		maskValue(instanceID), awsRegion, maskIPAddress(ipAddress))

	return map[string]string{
		instanceIdPlaceholder:    instanceID,
		hostnamePlaceholder:      hostname,
		localHostnamePlaceholder: localHostname,
		ipAddressPlaceholder:     ipAddress,
		awsRegionPlaceholder:     awsRegion,
		accountIdPlaceholder:     accountID,
	}
}

// maskValue masks sensitive values for logging
func maskValue(value string) string {
	if value == "" || value == unknownInstanceID {
		return "<empty>"
	}
	if len(value) <= 4 {
		return "<present>"
	}
	return value[:4] + "..."
}

// maskIPAddress masks IP addresses for logging (e.g., 10.0.x.x)
func maskIPAddress(ip string) string {
	if ip == "" || ip == unknownIPAddress {
		return "<empty>"
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".x.x"
	}
	return "<present>"
}

// isAzure detects if running on Azure
func isAzure() bool {
	if data, err := os.ReadFile("/sys/class/dmi/id/sys_vendor"); err == nil {
		if strings.Contains(strings.TrimSpace(string(data)), "Microsoft Corporation") {
			return true
		}
	}
	if data, err := os.ReadFile("/sys/class/dmi/id/chassis_asset_tag"); err == nil {
		if strings.TrimSpace(string(data)) == "7783-7084-3265-9085-8269-3286-77" {
			return true
		}
	}
	return false
}

// getAzureMetadataInfo returns metadata info for Azure
func getAzureMetadataInfo() map[string]string {
	localHostname := getHostName()
	ipAddress := getIpAddress()

	// Fetch from Azure IMDS
	azureMd := fetchAzureIMDS()

	instanceID := unknownInstanceID
	accountID := unknownAccountID
	region := unknownAwsRegion

	if azureMd != nil {
		instanceID = azureMd.VMID
		accountID = azureMd.ResourceGroupName
		region = "azure"
	}

	return map[string]string{
		instanceIdPlaceholder:    instanceID,
		hostnamePlaceholder:      localHostname,
		localHostnamePlaceholder: localHostname,
		ipAddressPlaceholder:     ipAddress,
		awsRegionPlaceholder:     region,
		accountIdPlaceholder:     accountID,
	}
}

func getAWSMetadataInfo(provider MetadataInfoProvider) map[string]string {
	md := provider()

	instanceID := defaultIfEmpty(md.InstanceID, unknownInstanceID)
	instanceType := defaultIfEmpty(md.InstanceType, unknownInstanceType)
	imageID := defaultIfEmpty(md.ImageID, unknownImageID)

	return map[string]string{
		ec2tagger.SupportedAppendDimensions[ec2tagger.MdKeyInstanceID]:   instanceID,
		ec2tagger.SupportedAppendDimensions[ec2tagger.MdKeyInstanceType]: instanceType,
		ec2tagger.SupportedAppendDimensions[ec2tagger.MdKeyImageID]:      imageID,
	}
}

var tagMetadataProvider func() map[string]string

func getTagMetadata() map[string]string {
	if tagMetadataProvider != nil {
		return tagMetadataProvider()
	}

	md := Ec2MetadataInfoProvider()
	instanceID := defaultIfEmpty(md.InstanceID, unknownInstanceID)

	if instanceID == unknownInstanceID {
		return map[string]string{}
	}

	result := make(map[string]string)
	asgName := tagutil.GetAutoScalingGroupName(instanceID)
	if asgName != "" {
		result[ec2tagger.SupportedAppendDimensions["AutoScalingGroupName"]] = asgName
	}
	return result
}

func getHostName() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	} else {
		log.Println("E! getHostName: ", err)
		return unknownHostname
	}
}

func getIpAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Println("E! getIpAddress -> getInterfaceAddrs: ", err)
		return unknownIPAddress
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return unknownIPAddress
}

func getAWSMetadata() map[string]string {
	return getAWSMetadataInfo(Ec2MetadataInfoProvider)
}

func getAWSMetadataWithTags(needsTags bool) map[string]string {
	metadata := getAWSMetadata()

	if needsTags {
		tagMetadata := getTagMetadata()
		for k, v := range tagMetadata {
			metadata[k] = v
		}
	}

	return metadata
}

func ResolveAWSMetadataPlaceholders(input any) any {
	inputMap := input.(map[string]interface{})
	result := make(map[string]any, len(inputMap))

	hasAWSPlaceholders := false
	needsTags := false

	for _, v := range inputMap {
		if vStr, ok := v.(string); ok && strings.Contains(vStr, awsPlaceholderPrefix) {
			hasAWSPlaceholders = true
			if vStr == ec2tagger.SupportedAppendDimensions["AutoScalingGroupName"] {
				needsTags = true
			}
		}
	}

	var metadata map[string]string
	if hasAWSPlaceholders {
		metadata = getAWSMetadataWithTags(needsTags)
	}

	for k, v := range inputMap {
		if vStr, ok := v.(string); ok && strings.Contains(vStr, awsPlaceholderPrefix) {
			if replacement, exists := metadata[vStr]; exists {
				result[k] = replacement
			}
		} else {
			result[k] = v
		}
	}
	return result
}

// ResolveAzureMetadataPlaceholders resolves Azure-specific placeholders like ${azure:InstanceId}
func ResolveAzureMetadataPlaceholders(input any) any {
	inputMap := input.(map[string]interface{})
	result := make(map[string]any, len(inputMap))

	hasAzurePlaceholders := false

	for _, v := range inputMap {
		if vStr, ok := v.(string); ok && strings.Contains(vStr, azurePlaceholderPrefix) {
			hasAzurePlaceholders = true
			break
		}
	}

	var metadata map[string]string
	if hasAzurePlaceholders {
		metadata = getAzureMetadata()
	}

	for k, v := range inputMap {
		if vStr, ok := v.(string); ok && strings.Contains(vStr, azurePlaceholderPrefix) {
			if replacement, exists := metadata[vStr]; exists {
				result[k] = replacement
			} else {
				log.Printf("W! Azure placeholder not found in metadata: %s", vStr)
				result[k] = v
			}
		} else {
			result[k] = v
		}
	}

	return result
}

// getAzureMetadata returns Azure metadata from IMDS
func getAzureMetadata() map[string]string {
	log.Println("D! [Azure Metadata] Fetching Azure IMDS metadata...")

	metadata := fetchAzureIMDS()
	if metadata == nil {
		log.Println("W! Failed to fetch Azure IMDS metadata, returning empty values")
		return map[string]string{
			"${azure:InstanceId}":        "",
			"${azure:InstanceType}":      "",
			"${azure:ImageId}":           "",
			"${azure:VmScaleSetName}":    "",
			"${azure:ResourceGroupName}": "",
		}
	}

	return map[string]string{
		"${azure:InstanceId}":        metadata.VMID,
		"${azure:InstanceType}":      metadata.VMSize,
		"${azure:ImageId}":           metadata.VMID,
		"${azure:VmScaleSetName}":    metadata.VMScaleSetName,
		"${azure:ResourceGroupName}": metadata.ResourceGroupName,
	}
}

// azureIMDSMetadata represents the Azure IMDS response we need
type azureIMDSMetadata struct {
	VMID              string
	VMSize            string
	VMScaleSetName    string
	ResourceGroupName string
}

// fetchAzureIMDS fetches metadata from Azure IMDS
func fetchAzureIMDS() *azureIMDSMetadata {
	log.Println("D! [Azure IMDS] Starting IMDS fetch...")

	client := &http.Client{Timeout: 2 * time.Second}

	req, err := http.NewRequest("GET", "http://169.254.169.254/metadata/instance/compute", nil)
	if err != nil {
		log.Printf("E! Failed to create Azure IMDS request: %v", err)
		return nil
	}

	req.Header.Add("Metadata", "true")
	q := req.URL.Query()
	q.Add("api-version", "2021-02-01")
	q.Add("format", "json")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("E! Azure IMDS HTTP request failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("E! Azure IMDS returned status code: %d", resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("E! Failed to read Azure IMDS response: %v", err)
		return nil
	}

	var result struct {
		VMID              string `json:"vmId"`
		VMSize            string `json:"vmSize"`
		VMScaleSetName    string `json:"vmScaleSetName"`
		ResourceGroupName string `json:"resourceGroupName"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("E! Failed to parse Azure IMDS JSON: %v", err)
		return nil
	}

	return &azureIMDSMetadata{
		VMID:              result.VMID,
		VMSize:            result.VMSize,
		VMScaleSetName:    result.VMScaleSetName,
		ResourceGroupName: result.ResourceGroupName,
	}
}

// ResolveCloudMetadataPlaceholders resolves both AWS and Azure placeholders
// Detects cloud provider and uses appropriate resolver
func ResolveCloudMetadataPlaceholders(input any) any {
	inputMap := input.(map[string]interface{})

	hasAzure := false
	hasAWS := false

	for _, v := range inputMap {
		if vStr, ok := v.(string); ok {
			if strings.Contains(vStr, azurePlaceholderPrefix) {
				hasAzure = true
			}
			if strings.Contains(vStr, awsPlaceholderPrefix) {
				hasAWS = true
			}
		}
	}

	result := input
	if hasAzure {
		result = ResolveAzureMetadataPlaceholders(result)
	}

	if hasAWS {
		result = ResolveAWSMetadataPlaceholders(result)
	}

	return result
}
