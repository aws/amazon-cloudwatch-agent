// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// Clean eks clusters if they have been open longer than 7 day
func main() {
	err := cleanAutoScalingGroups()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanAutoScalingGroups() error {
	log.Print("Begin to clean auto scaling groups Clusters")
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	autoScalingClient := autoscaling.NewFromConfig(defaultConfig)

	// filters are and statements not or thus run this 2 times
	// wilds cards do not work for asg tags unlike ec2
	ecsFilter := types.Filter{Name: aws.String("tag:BaseClusterName"), Values: []string{
		"cwagent-integ-test-cluster",
	}}
	eksFilter := types.Filter{Name: aws.String("tag:eks:nodegroup-name"), Values: []string{
		"cwagent-eks-integ-node",
	}}
	terminateAutoScaling(ctx, autoScalingClient, ecsFilter)
	terminateAutoScaling(ctx, autoScalingClient, eksFilter)
	return nil
}

func terminateAutoScaling(ctx context.Context, client *autoscaling.Client, filter types.Filter) {
	expirationDateCluster := time.Now().UTC().Add(clean.KeepDurationOneWeek)
	describeAutoScalingGroupsInput := autoscaling.DescribeAutoScalingGroupsInput{Filters: []types.Filter{
		filter,
	}}
	describeAutoScalingGroupsOutput, err := client.DescribeAutoScalingGroups(ctx, &describeAutoScalingGroupsInput)
	if err != nil {
		log.Fatalf("could not get auto scaling groups")
	}
	deletePass := 0
	deleteFail := 0
	for _, group := range describeAutoScalingGroupsOutput.AutoScalingGroups {
		if expirationDateCluster.After(*group.CreatedTime) {
			log.Printf("try to delete auto scaling group %s", *group.AutoScalingGroupName)
			deleteAutoScalingGroupInput := autoscaling.DeleteAutoScalingGroupInput{
				AutoScalingGroupName: group.AutoScalingGroupName,
				ForceDelete:          aws.Bool(true),
			}
			_, err := client.DeleteAutoScalingGroup(ctx, &deleteAutoScalingGroupInput)
			if err != nil {
				log.Printf("could not delete auto scaling group %s err %v", *group.AutoScalingGroupName, err)
				deleteFail++
			} else {
				deletePass++
			}
		}
	}
	log.Printf("was able to delete %d/%d for filter %v", deletePass, deletePass+deleteFail, filter)
}
