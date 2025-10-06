// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
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
)

// resolve place holder for log group and log stream.
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

// defaultIfEmpty returns defaultValue if value is empty, otherwise returns value
func defaultIfEmpty(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func GetMetadataInfo(provider MetadataInfoProvider) map[string]string {
	md := provider()
	localHostname := getHostName()

	instanceID := defaultIfEmpty(md.InstanceID, unknownInstanceID)
	hostname := defaultIfEmpty(md.Hostname, localHostname)
	ipAddress := defaultIfEmpty(md.PrivateIP, getIpAddress())
	awsRegion := defaultIfEmpty(agent.Global_Config.Region, unknownAwsRegion)
	accountID := defaultIfEmpty(md.AccountID, unknownAccountID)
	instanceType := defaultIfEmpty(md.InstanceType, unknownInstanceType)
	imageID := defaultIfEmpty(md.ImageID, unknownImageID)

	return map[string]string{
		// Standard placeholders
		instanceIdPlaceholder:    instanceID,
		hostnamePlaceholder:      hostname,
		localHostnamePlaceholder: localHostname,
		ipAddressPlaceholder:     ipAddress,
		awsRegionPlaceholder:     awsRegion,
		accountIdPlaceholder:     accountID,

		ec2tagger.SupportedAppendDimensions[ec2tagger.MdKeyInstanceID]:   instanceID,
		ec2tagger.SupportedAppendDimensions[ec2tagger.MdKeyInstanceType]: instanceType,
		ec2tagger.SupportedAppendDimensions[ec2tagger.MdKeyImageID]:      imageID,
	}
}

// GetAWSMetadataInfo returns AWS metadata using Ec2MetadataInfoProvider and EC2 Tags
func GetAWSMetadataInfo() map[string]string {
	// Start with the existing metadata pattern
	metadata := GetMetadataInfo(Ec2MetadataInfoProvider)

	// Add EC2 tags that require API calls (like AutoScaling group name)
	if asgName := GetEC2TagValue(ec2tagger.Ec2InstanceTagKeyASG); asgName != "" {
		metadata[ec2tagger.SupportedAppendDimensions[ec2tagger.CWDimensionASG]] = asgName
	}

	return metadata
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

// ResolveAWSMetadataPlaceholders resolves AWS metadata variables like ${aws:InstanceId} to actual values
func ResolveAWSMetadataPlaceholders(input any) any {
	awsMetadata := GetAWSMetadataInfo()
	inputMap := input.(map[string]interface{})
	result := make(map[string]any, len(inputMap))

	for k, v := range inputMap {
		vStr, isString := v.(string)
		if !isString {
			result[k] = v
			continue
		}

		resolvedValue := ResolvePlaceholder(vStr, awsMetadata)
		// Include if resolved successfully or if it's not an AWS variable
		if resolvedValue != vStr || !strings.Contains(vStr, "${aws:") {
			result[k] = resolvedValue
		}
		// Skip AWS variables that failed to resolve
	}
	return result
}
