// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build clean
// +build clean

package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithyTime "github.com/aws/smithy-go/time"
	"log"
	"time"
)

func main() {
	err := cleanAMI()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

const daysToKeep = 60
const keepDuration = -1 * time.Hour * 24 * time.Duration(daysToKeep)

var expirationDate = time.Now().UTC().Add(keepDuration)

func cleanAMI() []error {
	log.Print("Begin to clean EC2 AMI")

	cxt := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(cxt)
	if err != nil {
		return []error{err}
	}
	ec2client := ec2.NewFromConfig(defaultConfig)

	// Get list of ami
	nameFilter := types.Filter{Name: aws.String("name"), Values: []string{
		"cloudwatch-agent-integration-test*",
	}}

	//get instances to delete
	describeImagesInput := ec2.DescribeImagesInput{Filters: []types.Filter{nameFilter}}
	describeImagesOutput, err := ec2client.DescribeImages(cxt, &describeImagesInput)
	if err != nil {
		return []error{err}
	}

	var errors []error
	for _, image := range describeImagesOutput.Images {
		creationDate, err := smithyTime.ParseDateTime(*image.CreationDate)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		log.Printf("image name %v image id %v experation date %v creation date parsed %v image creation date raw %v",
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
		return errors
	}

	return nil
}
