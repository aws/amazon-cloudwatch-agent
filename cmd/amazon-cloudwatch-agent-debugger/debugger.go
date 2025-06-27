// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

var mergedConfig map[string]interface{}

func main() {

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--tarball":
			debugger.CreateTarball(false)
			return
		case "--tarballssm":
			debugger.CreateTarball(true)
			return
		}
	}

	ctx := context.Background()

	printHeader()
	info, err := debugger.GetInstanceInfo(ctx)
	if err != nil {
		log.Fatalf("Failed to get instance info: %v", err)
	}
	printInstanceInfo(info)

	debugger.CheckConfigFiles()

	//Load merged config, this is the same logic that the translator uses
	config, err := cmdutil.GetMergedConfig(paths.JsonConfigPath, paths.ConfigDirPath, "ec2", info.OS)
	if err != nil {
		log.Printf("Failed to load config: %v", err)
	} else {
		mergedConfig = config
		log.Println("\n=== Configuration Loaded ===")
		parseEndpoints()
	}

	debugger.CheckLogs(mergedConfig)
}

func printHeader() {
	log.Println("=== AWS EC2 Instance Information ===")
	log.Println()
}

func printInstanceInfo(info *debugger.InstanceInfo) {
	log.Println("")
	log.Printf("Instance ID:       %s\n", info.InstanceID)
	log.Printf("Account ID:        %s\n", info.AccountID)
	log.Printf("Region:            %s\n", info.Region)
	log.Printf("Instance Type:     %s\n", info.InstanceType)
	log.Printf("AMI:               %s\n", info.ImageID)
	log.Printf("Availability Zone: %s\n", info.AvailabilityZone)
	log.Printf("Architecture:      %s\n", info.Architecture)
	log.Printf("OS:				   %s\n", info.OS)
}

func parseEndpoints() {
	if mergedConfig == nil {
		log.Println("No configuration available")
		return
	}

	if metrics, ok := mergedConfig["metrics"].(map[string]interface{}); ok {
		if endpoint, ok := metrics["endpoint_override"].(string); ok {
			log.Printf("Metrics Endpoint: %s\n", endpoint)
		} else {
			log.Println("Metrics Endpoint: Default CloudWatch endpoint (no override)")
		}
	}

	if logs, ok := mergedConfig["logs"].(map[string]interface{}); ok {
		if endpoint, ok := logs["endpoint_override"].(string); ok {
			log.Printf("Logs Endpoint: %s\n", endpoint)
		} else {
			log.Println("Logs Endpoint: Default CloudWatch Logs endpoint (no override)")
		}
	}
}
