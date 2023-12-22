// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func GetAllAMIVersions(ec2Client *ec2.Client) []types.Image {
	//returns a sorted list by creation date
	filters := []types.Filter{
		//{ @TODO REMOVE
		//	Name:   aws.String("owner-id"),
		//	Values: []string{"self"},
		//},
		{
			Name:   aws.String("tag-key"),
			Values: []string{"build-env"},
		},
	}
	// Get the latest AMI made by your own user
	resp, err := ec2Client.DescribeImages(context.TODO(), &ec2.DescribeImagesInput{
		Filters: filters,
	})
	if err != nil {
		fmt.Println(err)
		return nil
	}
	// Sort the images based on the CreationDate field in descending order
	sort.Slice(resp.Images, func(i, j int) bool {
		return parseTime(*resp.Images[i].CreationDate).After(*parseTime(*resp.Images[j].CreationDate))
	})
	return resp.Images
}
func GetLatestAMIVersion(ec2Client *ec2.Client) *types.Image {
	amiList := GetAllAMIVersions(ec2Client)
	if len(amiList) > 0 {
		fmt.Println("Latest AMI ID:", *amiList[0].ImageId)
		return &amiList[0]
	} else {
		fmt.Println("No AMIs found.")
		return nil
	}
}
