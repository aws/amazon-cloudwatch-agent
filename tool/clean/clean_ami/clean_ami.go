// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build clean
// +build clean

package main

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithyTime "github.com/aws/smithy-go/time"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

var imagePrefixes = []string{
	"cloudwatch-agent-integration-test-mac*",
	"cloudwatch-agent-integration-test-al2*",
	"cloudwatch-agent-integration-test-ubuntu*",g
}

func main() {
	err := cleanAMIs()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

// takes a list of AMIs and sorts them by creation date (youngest to oldest)
func sortAMIsByCreationDate(amiList []types.Image, errList *[]error) []types.Image {
	sort.Slice(amiList, func(i, j int) bool {
		if amiList[i].CreationDate != nil && amiList[j].CreationDate != nil {
			iCreationDate, iErr := smithyTime.ParseDateTime(*amiList[i].CreationDate)
			jCreationDate, jErr := smithyTime.ParseDateTime(*amiList[j].CreationDate)

			if err := errors.Join(iErr, jErr); err != nil && errList != nil {
				*errList = append(*errList, err)
				return false
			}

			return iCreationDate.After(jCreationDate)
		} else {
			return false
		}
	})

	return amiList
}

// given a slice of AMIs, deregisters them one by one
func deregisterAMIs(ctx context.Context, ec2client *ec2.Client, images []types.Image, errList *[]error) {
	for _, image := range images {
		if image.Name != nil && image.ImageId != nil && image.CreationDate != nil {
			log.Printf("Try to delete ami %v tags %v image id %v image creation date raw %v", *image.Name, image.Tags, *image.ImageId, *image.CreationDate)
			deregisterImageInput := &ec2.DeregisterImageInput{ImageId: image.ImageId}
			_, err := ec2client.DeregisterImage(ctx, deregisterImageInput)

			if err != nil && errList != nil {
				log.Printf("Error while deregistering ami %v", *image.Name)
				*errList = append(*errList, err)
			}
		}
	}
}

// given a map of macos version/architecture to a list of corresponding AMIs, deregister AMIs that are no longer needed
func cleanMacAMIs(ctx context.Context, ec2client *ec2.Client, macosImageAmiMap map[string][]types.Image, expirationDate time.Time, errList *[]error) {
	for name, amiList := range macosImageAmiMap {
		// don't delete an ami if it's the only one for that version/architecture
		if len(amiList) == 1 {
			continue
		}

		// Sort AMIs by creation date (youngest to oldest)
		amiList = sortAMIsByCreationDate(amiList, errList)

		// find the youngest AMI in the list
		youngestCreationDate, err := smithyTime.ParseDateTime(aws.ToString(amiList[0].CreationDate))

		if err != nil && errList != nil {
			*errList = append(*errList, err)
			continue
		}

		if expirationDate.After(youngestCreationDate) {
			// If the youngest AMI is over 60 days old, we keep one (the youngest) and can delete the rest
			log.Printf("Youngest AMI for %s is over 60 days old. Deleting all but the youngest.", name)
			deregisterAMIs(ctx, ec2client, amiList[1:], errList)
		} else {
			// If the youngest AMI is under 60 days old, keep incrementing until we find AMIs older than 60 days and delete them
			for index, ami := range amiList {
				creationDate, err := smithyTime.ParseDateTime(aws.ToString(ami.CreationDate))
				if err != nil && errList != nil {
					*errList = append(*errList, err)
					continue
				}
				if expirationDate.After(creationDate) {
					// once you find the first AMI that's over 60 days old, delete the ones that follow
					deregisterAMIs(ctx, ec2client, amiList[index:], errList)
					break
				}
			}
		}
	}
}

// given a single non macos image, determine its age and deregister if needed
func cleanNonMacAMIs(ctx context.Context, ec2client *ec2.Client, image types.Image, expirationDate time.Time, errList *[]error) {
	creationDate, err := smithyTime.ParseDateTime(aws.ToString(image.CreationDate))
	if err != nil && errList != nil {
		*errList = append(*errList, err)
		return
	}

	if expirationDate.After(creationDate) {
		deregisterAMIs(ctx, ec2client, []types.Image{image}, errList)
	}
}

func cleanAMIs() error {
	log.Print("Begin to clean EC2 AMI")

	// sets expiration date to 60 days in the past
	expirationDate := time.Now().UTC().Add(clean.KeepDurationSixtyDay)
	log.Printf("Expiration date set as %v", expirationDate)

	// load default config
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	ec2client := ec2.NewFromConfig(defaultConfig)

	// stores a list of AMIs per each macos version/architecture
	macosImageAmiMap := make(map[string][]types.Image)

	// Cleanup for each AMI image type
	for _, filter := range imagePrefixes {
		nameFilter := types.Filter{Name: aws.String("name"), Values: []string{
			filter,
		}}

		//get instances to delete
		describeImagesInput := ec2.DescribeImagesInput{Filters: []types.Filter{nameFilter}}
		describeImagesOutput, err := ec2client.DescribeImages(ctx, &describeImagesInput)
		if err != nil {
			log.Printf("Image filter %s returned an error, skipping :%v", filter, err.Error())
			continue
		}

		if len(describeImagesOutput.Images) == 1 {
			log.Printf("Only 1 image found for filter %s, skipping", filter)
			continue
		}

		for _, image := range describeImagesOutput.Images {
			if image.Name != nil && strings.HasPrefix(*image.Name, "cloudwatch-agent-integration-test-mac") {
				// mac image - add it to the map and do nothing else for now
				macosImageAmiMap[*image.Name] = append(macosImageAmiMap[*image.Name], image)
			} else {
				// non mac image - clean it if it's older than 60 days
				cleanNonMacAMIs(ctx, ec2client, image, expirationDate, &errList)
			}
		}
	}

	// handle the mac AMIs
	cleanMacAMIs(ctx, ec2client, macosImageAmiMap, expirationDate, &errList)

	return nil
}

func getAMIsForFilter(string filter) ([]types.Image, error) {
	//get instances to delete
	describeImagesInput := ec2.DescribeImagesInput{Filters: []types.Filter{filter}}
	describeImagesOutput, err := ec2client.DescribeImages(ctx, &describeImagesInput)
	if err != nil {
		return []types.Image{}, err
	}

	return describeImagesOutput.Images, error
}
