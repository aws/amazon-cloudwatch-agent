// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

type Metadata struct {
	InstanceID string
	Hostname   string
	PrivateIP  string
	AccountID  string
}

type MetadataInfoProvider func() *Metadata

var Ec2MetadataInfoProvider = func() *Metadata {
	ec2 := ec2util.GetEC2UtilSingleton()
	return &Metadata{
		InstanceID: ec2.InstanceID,
		Hostname:   ec2.Hostname,
		PrivateIP:  ec2.PrivateIP,
		AccountID:  ec2.AccountID,
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

	unknownInstanceId = "i-UNKNOWN"
	unknownHostname   = "UNKNOWN-HOST"
	unknownIpAddress  = "UNKNOWN-IP"
	unknownAwsRegion  = "UNKNOWN-REGION"
	unknownAccountId  = "UNKNOWN-ACCOUNT"
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

	return map[string]string{instanceIdPlaceholder: instanceID, hostnamePlaceholder: hostname,
		localHostnamePlaceholder: localHostname, ipAddressPlaceholder: ipAddress, awsRegionPlaceholder: awsRegion,
		accountIdPlaceholder: accountID,
	}
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
