// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

func main() {
	err := cleanLaunchConfiguration()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}
func cleanLaunchConfiguration() error {
	expirationDate := time.Now().UTC().Add(clean.KeepDurationSixtyDay)
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	autoScalingClient := autoscaling.NewFromConfig(defaultConfig)
	describeLaunchConfigurationsInput := autoscaling.DescribeLaunchConfigurationsInput{}
	launchConfigOut, err := autoScalingClient.DescribeLaunchConfigurations(ctx, &describeLaunchConfigurationsInput)
	if err != nil {
		return err
	}
	if len(launchConfigOut.LaunchConfigurations) == 0 {
		return errors.New("no launch configuration found")
	}
	log.Printf("Found %d launch configurations", len(launchConfigOut.LaunchConfigurations))
	for _, launchConfig := range launchConfigOut.LaunchConfigurations {
		log.Printf("Found %s with creation date: %v", *launchConfig.LaunchConfigurationName, *launchConfig.CreatedTime)
		if expirationDate.After(*launchConfig.CreatedTime) {
			log.Printf("Try to delete %s", *launchConfig.LaunchConfigurationName)
			_, err := autoScalingClient.DeleteLaunchConfiguration(ctx, &autoscaling.DeleteLaunchConfigurationInput{
				LaunchConfigurationName: launchConfig.LaunchConfigurationName,
			})
			if err != nil {
				return err
			}
			log.Printf("Succesfully deleted %s", *launchConfig.LaunchConfigurationName)
		}
	}
	return nil
}
