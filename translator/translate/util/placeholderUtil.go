// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"log"
	"net"
	"os"
	"strings"
	"time"

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

	awsPlaceholderPrefix = "${aws:"
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
	md := provider()
	localHostname := getHostName()

	instanceID := defaultIfEmpty(md.InstanceID, unknownInstanceID)
	hostname := defaultIfEmpty(md.Hostname, localHostname)
	ipAddress := defaultIfEmpty(md.PrivateIP, getIpAddress())
	awsRegion := defaultIfEmpty(agent.Global_Config.Region, unknownAwsRegion)
	accountID := defaultIfEmpty(md.AccountID, unknownAccountID)

	return map[string]string{
		instanceIdPlaceholder:    instanceID,
		hostnamePlaceholder:      hostname,
		localHostnamePlaceholder: localHostname,
		ipAddressPlaceholder:     ipAddress,
		awsRegionPlaceholder:     awsRegion,
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
	asgName := tagutil.GetAutoScalingGroupName(context.Background(), instanceID)
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
