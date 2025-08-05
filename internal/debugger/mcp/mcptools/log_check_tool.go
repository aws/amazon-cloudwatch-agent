// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mcptools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

func NewLogCheckTool() mcp.Tool {
	return mcp.NewTool("log_check",
		mcp.WithDescription(`Validates configured log files and their accessibility for CloudWatch agent log collection.

		This tool examines all log files specified in the CloudWatch agent configuration,
		verifying their existence, readability, and accessibility for log collection. It
		parses the merged configuration from both JSON config files and configuration
		directory to identify all configured log sources.

		The function automatically detects the platform (Linux/Windows/macOS) and checks
		each configured log file path, including log groups, log streams, and any associated
		filtering or processing configurations.

		Usage: Use this tool to troubleshoot log collection issues, verify that configured
		log files are accessible, or diagnose why certain logs are not appearing in CloudWatch.

		Returns:
			LogConfigArray: An array containing detailed information about each configured
			log file's status and accessibility`))
}

// No arguments are required for this check. These parameters are here to match the interface.
// Errors are passed through NewToolResultError() and not through the error output. The error output is required for interface purposes.
func HandleLogCheck(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var outputBuffer bytes.Buffer

	config, err := cmdutil.GetMergedConfig(paths.JsonConfigPath, paths.ConfigDirPath, "ec2", runtime.GOOS)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error getting config: %s", err)), nil
	}

	logConfigArray, err := debugger.CheckConfiguredLogsExistsAndReadable(&outputBuffer, config, true)

	jsonResponse, err := json.Marshal(logConfigArray)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error marshaling JSON: %s", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil

}
