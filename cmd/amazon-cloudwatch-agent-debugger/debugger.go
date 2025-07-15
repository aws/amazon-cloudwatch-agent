// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger"
	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/mcp"
	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/utils"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

var mergedConfig map[string]interface{}

func main() {
	compact := flag.Bool("compact", false, "Run debugger with compact formatting")
	createtarball := flag.Bool("tarball", false, "Create tarball")
	createtarballssm := flag.Bool("tarballssm", false, "Create tarball with SSM")
	startmcpserver := flag.Bool("mcp", false, "Start MCP server for IDE integration")
	flag.Parse()

	switch {
	case *createtarball:
		debugger.CreateTarball(false)
		return
	case *createtarballssm:
		debugger.CreateTarball(true)
		return
	case *startmcpserver:
		mcp.StartMCPServer()
		return
	}

	defer func() {
		debugger.PrintAggregatedErrors()
		fmt.Println()
		fmt.Printf("If you are still unable to resolve your problem, refer to the CloudWatch Agent Troubleshooting docs: %s\n", "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/troubleshooting-CloudWatch-Agent.html")
	}()

	ctx := context.Background()

	printHeader()
	info, err := debugger.GetInstanceInfo(ctx)
	if err != nil {
		fmt.Printf("Failed to get instance info: %v\n", err)
	}
	printInstanceInfo(info, *compact)

	// We provide a stream because MCP uses a buffer. This is so when MCP calls tools it will not print to stdout.
	// There are better ways of doing this but not without significant refactoring overhead.
	if !debugger.CheckConfigFiles(os.Stdout, *compact) {
		fmt.Println("⚠️  ERROR: Required configuration files are missing - cannot conduct log checks.")
		return
	} else {
		config, err := cmdutil.GetMergedConfig(paths.JsonConfigPath, paths.ConfigDirPath, "ec2", info.OS)
		if err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			return
		} else {
			mergedConfig = config
		}
		_, err = debugger.CheckEndpoints(os.Stdout, mergedConfig, *compact)
		if err != nil {
			fmt.Printf("Failed to check endpoints: %v\n", err)
		}
		_, err = debugger.CheckLogs(os.Stdout, mergedConfig, *compact)
		if err != nil {
			fmt.Printf("Failed to check logs: %v\n", err)
			return
		}

	}

}

func printHeader() {
	fmt.Println("=== AWS EC2 Instance Information ===")
}

func printInstanceInfo(info *debugger.InstanceInfo, compact bool) {
	fmt.Println()

	values := []string{
		info.InstanceID,
		info.AccountID,
		info.Region,
		info.InstanceType,
		info.ImageID,
		info.AvailabilityZone,
		info.Architecture,
		info.OS,
		info.Version,
	}

	labelWidth := 18

	// Ensure minimum width for readability
	maxValueWidth := 15
	for _, v := range values {
		maxValueWidth = max(maxValueWidth, len(v))
	}

	if compact {
		fmt.Printf("Instance ID:       %s\n", info.InstanceID)
		fmt.Printf("Account ID:        %s\n", info.AccountID)
		fmt.Printf("Region:            %s\n", info.Region)
		fmt.Printf("Instance Type:     %s\n", info.InstanceType)
		fmt.Printf("AMI:               %s\n", info.ImageID)
		fmt.Printf("Availability Zone: %s\n", info.AvailabilityZone)
		fmt.Printf("Architecture:      %s\n", info.Architecture)
		fmt.Printf("OS:                %s\n", info.OS)
		fmt.Printf("Version:           %s\n", info.Version)
	} else {
		fmt.Printf("┌%s┬%s┐\n",
			utils.RepeatChar('─', labelWidth+2),
			utils.RepeatChar('─', maxValueWidth+2))

		printTableRow("Instance ID", info.InstanceID, labelWidth, maxValueWidth)
		printTableRow("Account ID", info.AccountID, labelWidth, maxValueWidth)
		printTableRow("Region", info.Region, labelWidth, maxValueWidth)
		printTableRow("Instance Type", info.InstanceType, labelWidth, maxValueWidth)
		printTableRow("AMI", info.ImageID, labelWidth, maxValueWidth)
		printTableRow("Availability Zone", info.AvailabilityZone, labelWidth, maxValueWidth)
		printTableRow("Architecture", info.Architecture, labelWidth, maxValueWidth)
		printTableRow("OS", info.OS, labelWidth, maxValueWidth)
		printTableRow("Version", info.Version, labelWidth, maxValueWidth)

		fmt.Printf("└%s┴%s┘\n",
			utils.RepeatChar('─', labelWidth+2),
			utils.RepeatChar('─', maxValueWidth+2))
	}

}

func printTableRow(label, value string, labelWidth, valueWidth int) {
	value = strings.TrimSpace(value)
	fmt.Printf("│ %-*s │ %-*s │\n", labelWidth, label, valueWidth, value)
}
