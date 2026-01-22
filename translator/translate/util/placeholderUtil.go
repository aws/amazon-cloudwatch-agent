// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata/azure"
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
		tmpString = strings.ReplaceAll(tmpString, k, v)
	}
	tmpString = strings.ReplaceAll(tmpString, datePlaceholder, time.Now().Format("2006-01-02"))
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
		cloudType := cloudmetadata.CloudProvider(cloudProvider.GetCloudProvider()).String()
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
			cloudmetadata.MaskValue(instanceID), hostname, region, cloudmetadata.MaskValue(accountID), cloudmetadata.MaskIPAddress(privateIP))

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
	if azure.IsAzure() {
		log.Printf("D! [placeholderUtil] cloudmetadata not available, using legacy Azure provider")
		return getAzureMetadataInfo()
	}

	// Fallback: AWS legacy path using provider function
	if provider == nil {
		log.Printf("W! [placeholderUtil] No provider available and cloudmetadata not initialized, using defaults")
		return map[string]string{
			instanceIdPlaceholder:    unknownInstanceID,
			hostnamePlaceholder:      localHostname,
			localHostnamePlaceholder: localHostname,
			ipAddressPlaceholder:     getIpAddress(),
			awsRegionPlaceholder:     unknownAwsRegion,
			accountIdPlaceholder:     unknownAccountID,
		}
	}
	log.Printf("D! [placeholderUtil] cloudmetadata not available, using legacy AWS provider")
	md := provider()

	instanceID := defaultIfEmpty(md.InstanceID, unknownInstanceID)
	hostname := defaultIfEmpty(md.Hostname, localHostname)
	ipAddress := defaultIfEmpty(md.PrivateIP, getIpAddress())
	awsRegion := defaultIfEmpty(agent.Global_Config.Region, unknownAwsRegion)
	accountID := defaultIfEmpty(md.AccountID, unknownAccountID)

	log.Printf("D! [placeholderUtil] Resolved via legacy: instanceId=%s, region=%s, privateIP=%s",
		cloudmetadata.MaskValue(instanceID), awsRegion, cloudmetadata.MaskIPAddress(ipAddress))

	return map[string]string{
		instanceIdPlaceholder:    instanceID,
		hostnamePlaceholder:      hostname,
		localHostnamePlaceholder: localHostname,
		ipAddressPlaceholder:     ipAddress,
		awsRegionPlaceholder:     awsRegion,
		accountIdPlaceholder:     accountID,
	}
}

// getAzureMetadataInfo returns metadata info for Azure
func getAzureMetadataInfo() map[string]string {
	localHostname := getHostName()
	ipAddress := getIpAddress()

	instanceID := unknownInstanceID
	accountID := unknownAccountID
	region := unknownAwsRegion

	// Try cloudmetadata provider first
	if provider := cloudmetadata.GetGlobalProviderOrNil(); provider != nil && provider.GetCloudProvider() == int(cloudmetadata.CloudProviderAzure) {
		if id := provider.GetInstanceID(); id != "" {
			instanceID = id
		}
		if acct := provider.GetAccountID(); acct != "" {
			accountID = acct
		}
		if reg := provider.GetRegion(); reg != "" {
			region = reg
		}
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
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		log.Printf("W! [placeholderUtil] ResolveAWSMetadataPlaceholders: input is not map[string]interface{}, returning unchanged")
		return input
	}
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
			// Support embedded placeholders: replace all occurrences in the string
			resolved := vStr
			for placeholder, replacement := range metadata {
				resolved = strings.ReplaceAll(resolved, placeholder, replacement)
			}
			// Only include if fully resolved (no placeholders remain)
			if !strings.Contains(resolved, awsPlaceholderPrefix) {
				result[k] = resolved
			}
			// Otherwise omit the key 
		} else {
			result[k] = v
		}
	}
	return result
}

// ResolveAzureMetadataPlaceholders resolves Azure-specific placeholders like ${azure:InstanceId}
func ResolveAzureMetadataPlaceholders(input any) any {
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		log.Printf("W! [placeholderUtil] ResolveAzureMetadataPlaceholders: input is not map[string]interface{}, returning unchanged")
		return input
	}
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
			// Support embedded placeholders: replace all occurrences in the string
			resolved := vStr
			for placeholder, replacement := range metadata {
				resolved = strings.ReplaceAll(resolved, placeholder, replacement)
			}
			// Only include if fully resolved (no placeholders remain)
			if !strings.Contains(resolved, azurePlaceholderPrefix) {
				result[k] = resolved
			}
			// Otherwise omit the key (backward compatible behavior)
		} else {
			result[k] = v
		}
	}

	return result
}

// getAzureMetadata returns Azure metadata from cloudmetadata provider
func getAzureMetadata() map[string]string {
	log.Println("D! [Azure Metadata] Fetching Azure metadata from cloudmetadata provider...")

	provider := cloudmetadata.GetGlobalProviderOrNil()
	if provider == nil || provider.GetCloudProvider() != int(cloudmetadata.CloudProviderAzure) {
		log.Println("W! Azure cloudmetadata provider not available, returning empty values")
		return map[string]string{
			"${azure:InstanceId}":        "",
			"${azure:InstanceType}":      "",
			"${azure:ImageId}":           "",
			"${azure:VmScaleSetName}":    "",
			"${azure:ResourceGroupName}": "",
		}
	}

	return map[string]string{
		"${azure:InstanceId}":        provider.GetInstanceID(),
		"${azure:InstanceType}":      provider.GetInstanceType(),
		"${azure:ImageId}":           provider.GetImageID(),
		"${azure:VmScaleSetName}":    provider.GetScalingGroupName(),
		"${azure:ResourceGroupName}": provider.GetResourceGroupName(),
	}
}

// ResolveCloudMetadataPlaceholders resolves both AWS and Azure placeholders
// Detects cloud provider and uses appropriate resolver
func ResolveCloudMetadataPlaceholders(input any) any {
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		log.Printf("W! [placeholderUtil] ResolveCloudMetadataPlaceholders: input is not map[string]interface{}, returning unchanged")
		return input
	}

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
