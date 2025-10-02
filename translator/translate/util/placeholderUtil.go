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

var Ec2MetadataInfoProvider = func() *Metadata {
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

	unknownInstanceId   = "i-UNKNOWN"
	unknownHostname     = "UNKNOWN-HOST"
	unknownIpAddress    = "UNKNOWN-IP"
	unknownAwsRegion    = "UNKNOWN-REGION"
	unknownAccountId    = "UNKNOWN-ACCOUNT"
	unknownInstanceType = "UNKNOWN-TYPE"
	unknownImageId      = "UNKNOWN-AMI"
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

func GetMetadataInfo(provider MetadataInfoProvider) map[string]string {
	localHostname := getHostName()

	instanceID := provider().InstanceID
	if instanceID == "" {
		instanceID = unknownInstanceId
	}

	hostname := provider().Hostname
	if hostname == "" {
		hostname = localHostname
	}

	ipAddress := provider().PrivateIP
	if ipAddress == "" {
		ipAddress = getIpAddress()
	}

	awsRegion := agent.Global_Config.Region
	if awsRegion == "" {
		awsRegion = unknownAwsRegion
	}

	accountID := provider().AccountID
	if accountID == "" {
		accountID = unknownAccountId
	}

	instanceType := provider().InstanceType
	if instanceType == "" {
		instanceType = unknownInstanceType
	}

	imageID := provider().ImageID
	if imageID == "" {
		imageID = unknownImageId
	}

	metadata := map[string]string{
		instanceIdPlaceholder:    instanceID,
		hostnamePlaceholder:      hostname,
		localHostnamePlaceholder: localHostname,
		ipAddressPlaceholder:     ipAddress,
		awsRegionPlaceholder:     awsRegion,
		accountIdPlaceholder:     accountID,
	}

	// Add AWS metadata placeholders
	metadata[ec2tagger.SupportedAppendDimensions["InstanceId"]] = instanceID
	metadata[ec2tagger.SupportedAppendDimensions["InstanceType"]] = instanceType
	metadata[ec2tagger.SupportedAppendDimensions["ImageId"]] = imageID

	return metadata
}

// GetAWSMetadataInfo returns AWS metadata using Ec2MetadataInfoProvider and EC2 Tags
func GetAWSMetadataInfo() map[string]string {
	// Start with the existing metadata pattern
	metadata := GetMetadataInfo(Ec2MetadataInfoProvider)

	// Add EC2 tags that require API calls (like AutoScaling group name)
	if asgName := GetEC2TagValue(ec2tagger.Ec2InstanceTagKeyASG); asgName != "" {
		metadata[ec2tagger.SupportedAppendDimensions["AutoScalingGroupName"]] = asgName
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
		return unknownIpAddress
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return unknownIpAddress
}

// ResolveAWSMetadataPlaceholders resolves AWS metadata variables like ${aws:InstanceId} to actual values
func ResolveAWSMetadataPlaceholders(input any) any {
	awsMetadata := GetAWSMetadataInfo()

	result := map[string]any{}
	for k, v := range input.(map[string]interface{}) {
		if vStr, ok := v.(string); ok {
			resolvedValue := ResolvePlaceholder(vStr, awsMetadata)
			if resolvedValue != vStr {
				result[k] = resolvedValue
			} else if !strings.Contains(vStr, "${aws:") {
				// Keep non-AWS variables as-is
				result[k] = v
			}
			// If AWS variable resolution fails, skip the dimension
		} else {
			result[k] = v
		}
	}
	return result
}
