// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

var mergedConfig map[string]interface{}

func main() {
	ssm := flag.Bool("ssm", false, "Run debugger with limited formatting for SSM")
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
	printInstanceInfo(info, *ssm)

	if !debugger.CheckConfigFiles(*ssm) {
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
		parseEndpoints(*ssm)
		_, err = debugger.CheckLogs(mergedConfig, *ssm)
		if err != nil {
			fmt.Printf("Failed to check logs: %v\n", err)
			return
		}

	}

}

func printHeader() {
	fmt.Println("=== AWS EC2 Instance Information ===")
}

func printInstanceInfo(info *debugger.InstanceInfo, ssm bool) {
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

	if ssm {
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
			repeatChar('─', labelWidth+2),
			repeatChar('─', maxValueWidth+2))

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
			repeatChar('─', labelWidth+2),
			repeatChar('─', maxValueWidth+2))
	}

}

func printTableRow(label, value string, labelWidth, valueWidth int) {
	value = strings.TrimSpace(value)
	fmt.Printf("│ %-*s │ %-*s │\n", labelWidth, label, valueWidth, value)
}

// Using runes to support "─"
func repeatChar(char rune, count int) string {
	result := make([]rune, count)
	for i := range result {
		result[i] = char
	}
	return string(result)
}

func parseEndpoints(ssm bool) {
	if mergedConfig == nil {
		fmt.Println("No configuration available")
		return
	}

	fmt.Println("\n=== Endpoint Configuration ===")

	var metricsEndpoint, logsEndpoint string
	if metrics, ok := mergedConfig["metrics"].(map[string]interface{}); ok {
		if endpoint, ok := metrics["endpoint_override"].(string); ok {
			metricsEndpoint = endpoint
		} else {
			metricsEndpoint = "Default CloudWatch endpoint (no override)"
		}
	} else {
		metricsEndpoint = "No metrics configuration found"
	}

	if logs, ok := mergedConfig["logs"].(map[string]interface{}); ok {
		if endpoint, ok := logs["endpoint_override"].(string); ok {
			logsEndpoint = endpoint
		} else {
			logsEndpoint = "Default CloudWatch Logs endpoint (no override)"
		}
	} else {
		logsEndpoint = "No logs configuration found"
	}

	if ssm {
		fmt.Printf("Metrics: %s\n", metricsEndpoint)
		fmt.Printf("Logs:    %s\n", logsEndpoint)
	} else {
		labelWidth := 15
		maxValueWidth := max(len(metricsEndpoint), len(logsEndpoint))
		maxValueWidth = max(maxValueWidth, 30)

		fmt.Printf("┌%s┬%s┐\n",
			repeatChar('─', labelWidth+2),
			repeatChar('─', maxValueWidth+2))

		printTableRow("Metrics", metricsEndpoint, labelWidth, maxValueWidth)
		printTableRow("Logs", logsEndpoint, labelWidth, maxValueWidth)

		fmt.Printf("└%s┴%s┘\n",
			repeatChar('─', labelWidth+2),
			repeatChar('─', maxValueWidth+2))
	}
}
