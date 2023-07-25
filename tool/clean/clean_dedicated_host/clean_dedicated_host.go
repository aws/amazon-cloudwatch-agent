// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build clean
// +build clean

package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// Can't release a host if it was being used within the last 24 hr add 2 hr as a buffer
const tagName = "tag:Name"
const tagValue = "IntegrationTestMacDedicatedHost"

func main() {
	err := cleanDedicatedHost()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanDedicatedHost() error {
	log.Print("Begin to clean EC2 Dedicated Host")

	expirationDateDedicatedHost := time.Now().UTC().Add(clean.KeepDurationTwentySixHours)
	cxt := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(cxt)
	if err != nil {
		return err
	}
	ec2client := ec2.NewFromConfig(defaultConfig)

	dedicatedHosts, err := getDedicatedHost(cxt, ec2client)
	if err != nil {
		return err
	}

	dedicatedHostIds := make([]string, 0)
	for _, dedicatedHost := range dedicatedHosts {
		log.Printf("dedicated host id %v experation date %v dedicated host creation date raw %v host state %v",
			*dedicatedHost.HostId, expirationDateDedicatedHost, *dedicatedHost.AllocationTime, dedicatedHost.State)
		if expirationDateDedicatedHost.After(*dedicatedHost.AllocationTime) && dedicatedHost.State == types.AllocationStateAvailable {
			log.Printf("Try to delete dedicated host %s tags %v launch-date %s", *dedicatedHost.HostId, dedicatedHost.Tags, *dedicatedHost.AllocationTime)
			dedicatedHostIds = append(dedicatedHostIds, *dedicatedHost.HostId)
		}
	}

	if len(dedicatedHostIds) == 0 {
		log.Printf("No dedicated hosts to release")
		return nil
	}

	log.Printf("Dedicated hosts to release %v", dedicatedHostIds)
	releaseDedicatedHost := ec2.ReleaseHostsInput{HostIds: dedicatedHostIds}
	_, err = ec2client.ReleaseHosts(cxt, &releaseDedicatedHost)
	return err
}

func getDedicatedHost(cxt context.Context, ec2client *ec2.Client) ([]types.Host, error) {
	// Get list of dedicated host
	nameFilter := types.Filter{Name: aws.String(tagName), Values: []string{
		tagValue,
	}}

	describeDedicatedHostInput := ec2.DescribeHostsInput{Filter: []types.Filter{nameFilter}}
	describeDedicatedHostOutput, err := ec2client.DescribeHosts(cxt, &describeDedicatedHostInput)
	if err != nil {
		return nil, err
	}
	return describeDedicatedHostOutput.Hosts, nil
}
