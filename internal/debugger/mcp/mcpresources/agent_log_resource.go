// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mcpresources

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

func NewLogResource() mcp.Resource {
	return mcp.NewResource(
		"file://agent.log",
		"CloudWatch Agent Log File",
		mcp.WithResourceDescription("The CloudWatch agent's log file"),
		mcp.WithMIMEType("text/plain"),
	)
}

func HandleAgentLogFile(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	agentLogFile, err := os.ReadFile(paths.AgentLogFilePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read agent log: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "text/plain",
			Text:     string(agentLogFile),
		},
	}, nil
}
