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
	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/utils"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

var mergedConfig map[string]interface{}

const (
	cwagentDocsUrl           = "https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/troubleshooting-CloudWatch-Agent.html"
	notPushingLogsDocsUrl    = "https://repost.aws/knowledge-center/cloudwatch-push-logs-with-unified-agent"
	notPushingMetricsDocsUrl = "https://repost.aws/knowledge-center/cloudwatch-unified-agent-metrics-issues"
)

func main() {
	compact := flag.Bool("compact", false, "Run debugger with compact formatting")
	createTarball := flag.Bool("tarball", false, "Create tarball")
	createTarballSsm := flag.Bool("tarballssm", false, "Create tarball with SSM")
	flag.Parse()

	switch {
	case *createTarball:
		debugger.CreateTarball(false)
		return
	case *createTarballSsm:
		debugger.CreateTarball(true)
		return
	}

	defer func() {
		debugger.GetErrorCollector().PrintErrors()
		fmt.Println()
		fmt.Println("If you are still unable to resolve your problem, refer to these docs:")
		fmt.Printf("General CWAgent troubleshooting docs: %s\n", cwagentDocsUrl)
		fmt.Printf("Logs not being pushed: %s\n", notPushingLogsDocsUrl)
		fmt.Printf("Metrics not being pushed: %s\n", notPushingMetricsDocsUrl)
	}()

	ctx := context.Background()

	printHeader()
	info, err := debugger.GetInstanceInfo(ctx)
	if err != nil {
		fmt.Printf("Failed to get instance info: %v\n", err)
	}
	printInstanceInfo(info, *compact)

	if !debugger.IsConfigFilesPresentAndReadable(os.Stdout, *compact) {
		fmt.Println("ERROR: There was an error collecting the config file - cannot conduct log and VPC endpoint checks.")
		return
	}

	configMap, err := cmdutil.GetMergedConfig(paths.JsonConfigPath, paths.ConfigDirPath, "ec2", info.OS)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	mergedConfig = configMap

	debugger.CheckLogsAndMetricsEndpoints(os.Stdout, mergedConfig, *compact)

	_, err = debugger.CheckConfiguredLogsExistsAndReadable(os.Stdout, mergedConfig, *compact)
	if err != nil {
		fmt.Printf("Failed to check logs: %v\n", err)
		return
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

	tableLabelColumnWidth := 18

	// Minimum width for readability
	tableValueColumnWidth := 15
	for _, v := range values {
		tableValueColumnWidth = max(tableValueColumnWidth, len(v))
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
			utils.RepeatChar('─', tableLabelColumnWidth+2),
			utils.RepeatChar('─', tableValueColumnWidth+2))

		printTableRow("Instance ID", info.InstanceID, tableLabelColumnWidth, tableValueColumnWidth)
		printTableRow("Account ID", info.AccountID, tableLabelColumnWidth, tableValueColumnWidth)
		printTableRow("Region", info.Region, tableLabelColumnWidth, tableValueColumnWidth)
		printTableRow("Instance Type", info.InstanceType, tableLabelColumnWidth, tableValueColumnWidth)
		printTableRow("AMI", info.ImageID, tableLabelColumnWidth, tableValueColumnWidth)
		printTableRow("Availability Zone", info.AvailabilityZone, tableLabelColumnWidth, tableValueColumnWidth)
		printTableRow("Architecture", info.Architecture, tableLabelColumnWidth, tableValueColumnWidth)
		printTableRow("OS", info.OS, tableLabelColumnWidth, tableValueColumnWidth)
		printTableRow("Version", info.Version, tableLabelColumnWidth, tableValueColumnWidth)

		fmt.Printf("└%s┴%s┘\n",
			utils.RepeatChar('─', tableLabelColumnWidth+2),
			utils.RepeatChar('─', tableValueColumnWidth+2))
	}

}

func printTableRow(label, value string, labelWidth, valueWidth int) {
	value = strings.TrimSpace(value)
	fmt.Printf("│ %-*s │ %-*s │\n", labelWidth, label, valueWidth, value)
}
