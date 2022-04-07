// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build clean
// +build clean

package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/ssm"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithyTime "github.com/aws/smithy-go/time"
	"log"
	"time"
)

const (
	daysToKeep = 30
	keepDuration = -1 * time.Hour * 24 * time.Duration(daysToKeep)
	expirationDate = time.Now().UTC().Add(keepDuration)
)

func main() {
	log.Println("Begin to clean EC2 AMI")
	//cleanAMI()

	log.Println("Begin to clean SSM Parameter Store")
	cleanSSMParameterStore()

	log.Println("Finished cleaning resources.")
}

func cleanSSMParameterStore() {
	cxt := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(cxt)

	if err != nil {
		log.Printf("Load default config failed because of %v",err)
		return
	}

	ssmClient := ssm.NewFromConfig(defaultConfig)

	//Allow to load all th since the default respond is paginated auto scaling groups.
	//Look into the documentations and read the starting-token for more details
	//Documentation: https://docs.aws.amazon.com/cli/latest/reference/autoscaling/describe-auto-scaling-groups.html#options
	var nextToken *string
	var errors []error

	for {
		describeParametersInput := ssm.DescribeParametersInput{}
		describeParametersOutput, err := ec2client.DescribeImages(cxt, &describeParametersInput)
		if err != nil {
			return []error{err}
		}

		for _, asg := range describeParametersOutput.AutoScalingGroups {

			//Skipping Store Parameters that does not older than 1 months
			if !expirationDate.After(*asg.CreatedTime) {
				continue
			}

			deleteAutoScalingGroupInput := &autoscaling.DeleteAutoScalingGroupInput{
				AutoScalingGroupName: asg.AutoScalingGroupName,
				ForceDelete:          aws.Bool(true),
			}

			_, err = autoscalingclient.DeleteAutoScalingGroup(deleteAutoScalingGroupInput)

			if err != nil {
				return err
			}

			deleteLaunchConfigurationInput := &autoscaling.DeleteLaunchConfigurationInput{
				LaunchConfigurationName: asg.LaunchConfigurationName,
			}

			if _, err = autoscalingclient.DeleteLaunchConfiguration(deleteLaunchConfigurationInput); err != nil {
				return err
			}

			logger.Printf("Deleted asg %s successfully", *asg.AutoScalingGroupName)
		}

		if describeAutoScalingOutputs.NextToken == nil {
			break
		}

		nextToken = describeImagesOutput.NextToken
	}

}

func cleanAMI() {
	cxt := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(cxt)

	if err != nil {
		log.Printf("Load default config failed because of %v",err)
		return
	}

	ec2client := ec2.NewFromConfig(defaultConfig)

	// Get list of ami
	nameFilter := types.Filter{Key: aws.String("name"), Values: []string{
		"cloudwatch-agent-integration-test*",
	}}

	//get instances to delete
	describeImagesInput := ec2.DescribeImagesInput{}
	describeImagesOutput, err := ec2client.DescribeImages(cxt, &describeImagesInput)
	if err != nil {
		log.Printf("Describe images failed because of %v",err)
		return
	}

	var errors []error
	for _, image := range describeImagesOutput.Images {
		creationDate, err := smithyTime.ParseDateTime(*image.CreationDate)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		log.Printf("Image name %v image id %v experation date %v creation date parsed %v image creation date raw %v",
			*image.Name, *image.ImageId, creationDate, expirationDate, *image.CreationDate)
		if expirationDate.After(creationDate) {
			log.Printf("Try to delete ami %s tags %v launch-date %s", *image.Name, image.Tags, *image.CreationDate)
			deregisterImageInput := ec2.DeregisterImageInput{ImageId: image.ImageId}
			_, err := ec2client.DeregisterImage(cxt, &deregisterImageInput)
			if err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) != 0 {
		log.Printf("Deleted some of AMIs failed because of %v", err)
		return
	}
}
