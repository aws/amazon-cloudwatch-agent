// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build clean
// +build clean

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithyTime "github.com/aws/smithy-go/time"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

func main() {
	err := cleanAMI()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanAMI() error {
	log.Print("Begin to clean EC2 AMI")

	expirationDate := time.Now().UTC().Add(clean.KeepDurationSixtyDay)
	cxt := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(cxt)
	if err != nil {
		return err
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
		return err
	}

	var errList []error
	for _, image := range describeImagesOutput.Images {
		creationDate, err := smithyTime.ParseDateTime(*image.CreationDate)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		log.Printf("image name %v image id %v experation date %v creation date parsed %v image creation date raw %v",
			*image.Name, *image.ImageId, creationDate, expirationDate, *image.CreationDate)
		if expirationDate.After(creationDate) {
			log.Printf("Try to delete ami %s tags %v launch-date %s", *image.Name, image.Tags, *image.CreationDate)
			deregisterImageInput := ec2.DeregisterImageInput{ImageId: image.ImageId}
			_, err := ec2client.DeregisterImage(cxt, &deregisterImageInput)
			if err != nil {
				errList = append(errList, err)
			}
		}
	}

	if len(errList) != 0 {
		return fmt.Errorf("%v", errList)
	}

	return nil
}
