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
	ec2Type "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmType "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	smithyTime "github.com/aws/smithy-go/time"
)

const (
	daysToKeep   = 30
	keepDuration = -1 * time.Hour * 24 * time.Duration(daysToKeep)
)

var expirationDate = time.Now().UTC().Add(keepDuration)

func main() {
	log.Println("Begin to clean EC2 AMI")
	cleanAMI()

	log.Println("Begin to clean SSM Parameter Store")
	cleanSSMParameterStore()

	log.Println("Finished cleaning resources.")
}

func cleanSSMParameterStore() {
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		log.Printf("Load default config failed because of %v", err)
		return
	}

	ssmClient := ssm.NewFromConfig(defaultConfig)

	//Allow to load all th since the default respond is paginated auto scaling groups.
	//Look into the documentations and read the starting-token for more details
	//Documentation: https://docs.aws.amazon.com/cli/latest/reference/autoscaling/describe-auto-scaling-groups.html#options
	var nextToken *string

	var parameterStoreNameFilter = ssmType.ParameterStringFilter{
		Key:    aws.String("Name"),
		Option: aws.String("BeginsWith"),
		Values: []string{"AmazonCloudWatch"},
	}

	for {
		describeParametersInput := ssm.DescribeParametersInput{
			ParameterFilters: []ssmType.ParameterStringFilter{parameterStoreNameFilter},
			NextToken:        nextToken,
		}
		describeParametersOutput, err := ssmClient.DescribeParameters(ctx, &describeParametersInput)

		if err != nil {
			log.Printf("Describe Parameter Stores failed because of %v", err)
			return
		}

		for _, parameter := range describeParametersOutput.Parameters {

			if !expirationDate.After(*parameter.LastModifiedDate) {
				continue
			}

			log.Printf("Trying to delete Parameter Store with name %s and creation date %v", *parameter.Name, *parameter.LastModifiedDate)

			deleteParameterInput := ssm.DeleteParameterInput{Name: parameter.Name}

			if _, err := ssmClient.DeleteParameter(ctx, &deleteParameterInput); err != nil {
				log.Printf("Failed to delete Parameter Store with name %s because of %v", *parameter.Name, err)
				return
			}
		}

		if describeParametersOutput.NextToken == nil {
			break
		}

		nextToken = describeParametersOutput.NextToken
	}
}

func cleanAMI() {
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		log.Printf("Load default config failed because of %v", err)
		return
	}

	ec2client := ec2.NewFromConfig(defaultConfig)

	// Filter name of EC2
	ec2NameFilter := ec2Type.Filter{Name: aws.String("name"), Values: []string{
		"cloudwatch-agent-integration-test*",
	}}

	//get instances to delete
	describeImagesInput := ec2.DescribeImagesInput{Filters: []ec2Type.Filter{ec2NameFilter}}
	describeImagesOutput, err := ec2client.DescribeImages(ctx, &describeImagesInput)
	if err != nil {
		log.Printf("Describe images failed because of %v", err)
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
			_, err := ec2client.DeregisterImage(ctx, &deregisterImageInput)
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
