// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// Clean ebs volumes if they have been open longer than 7 day and unused
func main() {
	err := cleanVolumes()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanVolumes() error {
	log.Print("Begin to clean EBS Volumes")
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	ec2Client := ec2.NewFromConfig(defaultConfig)

	return deleteUnusedVolumes(ctx, ec2Client)

}

func deleteUnusedVolumes(ctx context.Context, client *ec2.Client) error {

	input := &ec2.DescribeVolumesInput{
		Filters: []types.Filter{
			{
				//if the status is availble, then EBS volume is not currently attached to any ec2 instance (so not being used)
				Name:   aws.String("status"),
				Values: []string{"available"},
			},
		},
	}

	volumes, err := client.DescribeVolumes(ctx, input)
	if err != nil {
		return err
	}
	for _, volume := range volumes.Volumes {
		if time.Since(*volume.CreateTime) > clean.KeepDurationOneWeek && len(volume.Attachments) == 0 {
			log.Printf("Deleting unused volume %s", *volume.VolumeId)
			_, err = client.DeleteVolume(ctx, &ec2.DeleteVolumeInput{
				VolumeId: volume.VolumeId,
			})
		}
		if err != nil {
			log.Printf("Error deleting volume %s: %v", *volume.VolumeId, err)
		}
	}
	return nil
}
