// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// Clean integration hosts if they have been open longer than 1 day
func main() {
	err := cleanHost()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanHost() error {
	log.Print("Begin to clean EC2 Host")

	cxt := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(cxt, config.WithRegion(os.Args[1]))
	if err != nil {
		return err
	}
	ec2client := ec2.NewFromConfig(defaultConfig)

	terminateInstances(cxt, ec2client)
	return err
}

func terminateInstances(cxt context.Context, ec2client *ec2.Client) {
	maxResults := int32(1000)
	nameFilter := types.Filter{Name: aws.String("tag:Name"), Values: []string{
		"buildLinuxPackage",
		"buildPKG",
		"buildMSI",
		"MSIUpgrade_*",
		"Ec2IntegrationTest",
		"IntegrationTestBase",
		"CWADockerImageBuilderX86",
		"CWADockerImageBuilderARM64",
		"cwagent-integ-test-ec2*",
		"cwagent-integ-test-ec2-windows-*",
		"cwagent-performance-*",
		"cwagent-stress-*",
		"LocalStackIntegrationTestInstance",
		"NvidiaDataCollector-*",
	}}

	instanceInput := ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			nameFilter,
			{Name: aws.String("instance-state-name"),
				Values: []string{"running"}}},
		MaxResults: aws.Int32(maxResults)}
	for {
		instanceIds := make([]string, 0)
		expirationDateInstance := time.Now().UTC().Add(clean.KeepDurationOneDay)
		describeInstanceOutput, _ := ec2client.DescribeInstances(cxt, &instanceInput)
		for _, reservation := range describeInstanceOutput.Reservations {
			for _, instance := range reservation.Instances {
				log.Printf("instance id %v expiration date %v host creation date raw %v host state %v",
					*instance.InstanceId, expirationDateInstance, *instance.LaunchTime, instance.State)
				if expirationDateInstance.After(*instance.LaunchTime) {
					log.Printf("Try to delete instance %s tags %v launch-date %s", *instance.InstanceId, instance.Tags, *instance.LaunchTime)
					instanceIds = append(instanceIds, *instance.InstanceId)
				}
			}
		}
		if len(instanceIds) == 0 {
			log.Printf("No instances to terminate")
			return
		}

		log.Printf("instances to terminate %v", instanceIds)
		terminateInstance := ec2.TerminateInstancesInput{InstanceIds: instanceIds}
		_, err := ec2client.TerminateInstances(cxt, &terminateInstance)
		if err != nil {
			log.Printf("Error %v terminating instances %v", err, instanceIds)
		}
		if describeInstanceOutput.NextToken == nil {
			break
		}
		instanceInput.NextToken = describeInstanceOutput.NextToken
		// prevent throttle https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html
		time.Sleep(time.Minute)
	}
}
