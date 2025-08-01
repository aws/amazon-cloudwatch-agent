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
		mcp.WithDescription("Check the exitence and readability of configured logs for the CloudWatch agent."))
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
