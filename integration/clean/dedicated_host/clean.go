// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dedicated_host

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Can't release a host if it was being used within the last 24 hr add 2 hr as a buffer
const (
	Type     = "dedicated_host"
	tagName  = "tag:Name"
	tagValue = "IntegrationTestMacDedicatedHost"
)

func Clean(ctx context.Context, expirationDate time.Time) error {
	log.Print("Begin to clean EC2 Dedicated Host")

	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	ec2client := ec2.NewFromConfig(defaultConfig)
	dedicatedHosts, err := getDedicatedHost(ctx, ec2client)
	if err != nil {
		return err
	}

	dedicatedHostIds := make([]string, 0)
	for _, dedicatedHost := range dedicatedHosts {
		log.Printf("dedicated host id %v experation date %v dedicated host creation date raw %v host state %v",
			*dedicatedHost.HostId, expirationDate, *dedicatedHost.AllocationTime, dedicatedHost.State)
		isDedicatedHostAvailableOrPending := dedicatedHost.State == types.AllocationStateAvailable || dedicatedHost.State == types.AllocationStatePending
		if expirationDate.After(*dedicatedHost.AllocationTime) && len(dedicatedHost.Instances) == 0 && isDedicatedHostAvailableOrPending {
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
	_, err = ec2client.ReleaseHosts(ctx, &releaseDedicatedHost)

	if err != nil {
		return err
	}

	log.Println("Finished cleaning dedicated host")
	return nil
}

func getDedicatedHost(ctx context.Context, ec2client *ec2.Client) ([]types.Host, error) {
	// Get list of dedicated host
	nameFilter := types.Filter{Name: aws.String(tagName), Values: []string{
		tagValue,
	}}

	describeDedicatedHostInput := ec2.DescribeHostsInput{Filter: []types.Filter{nameFilter}}
	describeDedicatedHostOutput, err := ec2client.DescribeHosts(ctx, &describeDedicatedHostInput)
	if err != nil {
		return nil, err
	}
	return describeDedicatedHostOutput.Hosts, nil
}
