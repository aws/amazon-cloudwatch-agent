// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

var mergedConfig map[string]interface{}

func main() {
	tarball := flag.Bool("tarball", false, "Create tarball")
	tarballssm := flag.Bool("tarballssm", false, "Create tarball with SSM")
	flag.Parse()

	switch {
	case *tarball:
		debugger.CreateTarball(false)
		return
	case *tarballssm:
		debugger.CreateTarball(true)
		return
	}

	ctx := context.Background()

	printHeader()
	info, err := debugger.GetInstanceInfo(ctx)
	if err != nil {
		fmt.Printf("Failed to get instance info: %v\n", err)
	}
	printInstanceInfo(info)

	debugger.CheckConfigFiles()

	//Load merged config, this is the same logic that the translator uses
	config, err := cmdutil.GetMergedConfig(paths.JsonConfigPath, paths.ConfigDirPath, "ec2", info.OS)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
	} else {
		mergedConfig = config
		fmt.Println("\n=== Configuration Loaded ===")
		parseEndpoints()
	}

	debugger.CheckLogs(mergedConfig)
}

func printHeader() {
	fmt.Println("=== AWS EC2 Instance Information ===")
	fmt.Println()
}

func printInstanceInfo(info *debugger.InstanceInfo) {
	fmt.Println("")
	fmt.Printf("Instance ID:       %s\n", info.InstanceID)
	fmt.Printf("Account ID:        %s\n", info.AccountID)
	fmt.Printf("Region:            %s\n", info.Region)
	fmt.Printf("Instance Type:     %s\n", info.InstanceType)
	fmt.Printf("AMI:               %s\n", info.ImageID)
	fmt.Printf("Availability Zone: %s\n", info.AvailabilityZone)
	fmt.Printf("Architecture:      %s\n", info.Architecture)
	fmt.Printf("OS:				   %s\n", info.OS)
	fmt.Printf("Version:           %s\n", info.Version)
}

func parseEndpoints() {
	if mergedConfig == nil {
		fmt.Println("No configuration available")
		return
	}

	if metrics, ok := mergedConfig["metrics"].(map[string]interface{}); ok {
		if endpoint, ok := metrics["endpoint_override"].(string); ok {
			fmt.Printf("Metrics Endpoint: %s\n", endpoint)
		} else {
			fmt.Println("Metrics Endpoint: Default CloudWatch endpoint (no override)")
		}
	}

	if logs, ok := mergedConfig["logs"].(map[string]interface{}); ok {
		if endpoint, ok := logs["endpoint_override"].(string); ok {
			fmt.Printf("Logs Endpoint: %s\n", endpoint)
		} else {
			fmt.Println("Logs Endpoint: Default CloudWatch Logs endpoint (no override)")
		}
	}
}
