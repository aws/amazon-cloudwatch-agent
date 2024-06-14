// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
)

type ec2Info struct {
	InstanceID       string
	AutoScalingGroup string

	// region is used while making call to describeTags Ec2 API for AutoScalingGroup
	Region string

	metadataProvider ec2metadataprovider.MetadataProvider
	ec2API           ec2iface.EC2API
	ec2Provider      ec2ProviderType
	shutdownC        chan bool
}

func (ei *ec2Info) initEc2Info() {
	log.Println("I! ec2Info: Initializing ec2Info")
	ei.shutdownC = make(chan bool)
	if err := ei.setInstanceIdAndRegion(); err != nil {
		return
	}
	ei.ec2API = ei.ec2Provider(ei.Region)
	if err := ei.setAutoScalingGroup(); err != nil {
		return
	}
	log.Printf("D! ec2Info: Finished initializing ec2Info: InstanceId %s, AutoScalingGroup %s", ei.InstanceID, ei.AutoScalingGroup)
	ei.Shutdown()
}

func (ei *ec2Info) setInstanceIdAndRegion() error {
	for {
		metadataDoc, err := ei.metadataProvider.Get(context.Background())
		if err != nil {
			log.Printf("E! ec2Info: Failed to get Instance Id and region through metadata provider: %v", err)
			wait := time.NewTimer(1 * time.Minute)
			select {
			case <-ei.shutdownC:
				wait.Stop()
				return errors.New("ec2Info: shutdownC received")
			case <-wait.C:
			}
		} else {
			ei.InstanceID = metadataDoc.InstanceID
			ei.Region = metadataDoc.Region
			log.Printf("D! ec2Info: Successfully retrieved Instance Id %s, Region %s", ei.InstanceID, ei.Region)
			return nil
		}
	}
}

func (ei *ec2Info) setAutoScalingGroup() error {
	retry := 0
	for {
		var waitDuration time.Duration
		if retry < len(ec2tagger.BackoffSleepArray) {
			waitDuration = ec2tagger.BackoffSleepArray[retry]
		} else {
			waitDuration = ec2tagger.BackoffSleepArray[len(ec2tagger.BackoffSleepArray)-1]
		}

		wait := time.NewTimer(waitDuration)
		select {
		case <-ei.shutdownC:
			wait.Stop()
			return errors.New("ec2Info: shutdownC received")
		case <-wait.C:
		}

		if retry > 0 {
			log.Printf("D! ec2Info: initial retrieval of tags and volumes with retry: %d", retry)
		}

		if err := ei.retrieveAsgName(ei.ec2API); err != nil {
			log.Printf("E! ec2Info: Unable to describe ec2 tags for retry %d with error %v", retry, err)
		} else {
			log.Println("I! ec2Info: Retrieval of tags succeeded")
			return nil
		}

		retry++
	}

}

/*
This can also be implemented by just calling the InstanceTagValue and then DescribeTags on failure. But preferred the current implementation
as we need to distinguish the tags not being fetchable at all, from the ASG tag in particular not existing.
*/
func (ei *ec2Info) retrieveAsgName(ec2API ec2iface.EC2API) error {
	tags, err := ei.metadataProvider.InstanceTags(context.Background())
	if err != nil {
		log.Printf("E! ec2Info: Failed to get tags through metadata provider: %v", err.Error())
		return ei.retrieveAsgNameWithDescribeTags(ec2API)
	} else if strings.Contains(tags, ec2tagger.Ec2InstanceTagKeyASG) {
		asg, err := ei.metadataProvider.InstanceTagValue(context.Background(), ec2tagger.Ec2InstanceTagKeyASG)
		if err != nil {
			log.Printf("E! ec2Info: Failed to get AutoScalingGroup through metadata provider: %v", err.Error())
		} else {
			log.Printf("D! ec2Info: AutoScalingGroup retrieved through IMDS: %s", asg)
			ei.AutoScalingGroup = asg
		}
	}
	return nil
}

func (ei *ec2Info) retrieveAsgNameWithDescribeTags(ec2API ec2iface.EC2API) error {
	tagFilters := []*ec2.Filter{
		{
			Name:   aws.String("resource-type"),
			Values: aws.StringSlice([]string{"instance"}),
		},
		{
			Name:   aws.String("resource-id"),
			Values: aws.StringSlice([]string{ei.InstanceID}),
		},
		{
			Name:   aws.String("key"),
			Values: aws.StringSlice([]string{ec2tagger.Ec2InstanceTagKeyASG}),
		},
	}
	input := &ec2.DescribeTagsInput{
		Filters: tagFilters,
	}
	for {
		result, err := ec2API.DescribeTags(input)
		if err != nil {
			log.Println("E! ec2Info: Unable to retrieve EC2 AutoScalingGroup. This feature must only be used on an EC2 instance.")
			return err
		}
		for _, tag := range result.Tags {
			key := *tag.Key
			if ec2tagger.Ec2InstanceTagKeyASG == key {
				ei.AutoScalingGroup = *tag.Value
				return nil
			}
		}
		if result.NextToken == nil {
			break
		}
		input.SetNextToken(*result.NextToken)
	}
	return nil
}

func (ei *ec2Info) Shutdown() {
	close(ei.shutdownC)
}

func newEC2Info(metadataProvider ec2metadataprovider.MetadataProvider, providerType ec2ProviderType) *ec2Info {
	return &ec2Info{
		metadataProvider: metadataProvider,
		ec2Provider:      providerType,
	}
}
