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

func main() {
	var expirationDate = time.Now().UTC().Add(keepDuration)

	log.Println("Begin to clean AWS resources.")

	cleanAMI(expirationDate)
	cleanSSMParameterStore(expirationDate)

	log.Println("Finished cleaning AWS resources.")
}

func cleanSSMParameterStore(expirationDate time.Time) {
	log.Println("Begin to clean SSM Parameter Store")

	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		log.Fatalf("Load default config failed because of %v", err)
	}

	ssmClient := ssm.NewFromConfig(defaultConfig)

	//Allow to load all th since the default respond is paginated auto scaling groups.
	//Look into the documentations and read the starting-token for more details
	//Documentation: https://docs.aws.amazon.com/cli/latest/reference/autoscaling/describe-auto-scaling-groups.html#options
	var nextToken *string
	var errors []error

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
			log.Fatalf("Describe Parameter Stores failed because of %v", err)
		}

		for _, parameter := range describeParametersOutput.Parameters {

			if !expirationDate.After(*parameter.LastModifiedDate) {
				continue
			}

			log.Printf("Trying to delete Parameter Store with name %s and creation date %v", *parameter.Name, *parameter.LastModifiedDate)

			deleteParameterInput := ssm.DeleteParameterInput{Name: parameter.Name}

			if _, err := ssmClient.DeleteParameter(ctx, &deleteParameterInput); err != nil {
				errors = append(errors, err)
			}
		}

		if describeParametersOutput.NextToken == nil {
			break
		}

		nextToken = describeParametersOutput.NextToken
	}

	if len(errors) != 0 {
		log.Fatalf("Deleted some of Parameter Store failed because of %v", err)
	}
	
	log.Println("End cleaning SSM Parameter Store")
}

func cleanAMI(expirationDate time.Time) {
	log.Println("Begin to clean EC2 AMI")

	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		log.Fatalf("Load default config failed because of %v", err)
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
		log.Fatalf("Describe images failed because of %v", err)
	}

	var errors []error
	for _, image := range describeImagesOutput.Images {
		creationDate, err := smithyTime.ParseDateTime(*image.CreationDate)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		if !expirationDate.After(creationDate) {
			continue
		}
		
		log.Printf("Try to delete ami %s with image id %s, tags %v launch-date %s", *image.Name,  *image.ImageId, image.Tags, *image.CreationDate)
		
		deregisterImageInput := ec2.DeregisterImageInput{ImageId: image.ImageId}
		
		if _, err := ec2client.DeregisterImage(ctx, &deregisterImageInput); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) != 0 {
		log.Fatalf("Deleted some of AMIs failed because of %v", err)
	}

	log.Println("End cleaning EC2 AMI")
}
