// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ami

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithyTime "github.com/aws/smithy-go/time"
)

const (
	Type = "ami"
)

func Clean(ctx context.Context, expirationDate time.Time) error {
	log.Print("Begin to clean EC2 AMI")

	defaultConfig, err := config.LoadDefaultConfig(ctx)
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
	describeImagesOutput, err := ec2client.DescribeImages(ctx, &describeImagesInput)
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
			_, err := ec2client.DeregisterImage(ctx, &deregisterImageInput)
			if err != nil {
				return err
			}
		}
	}

	log.Println("Finished cleaning EC2 AMI")
	return nil
}