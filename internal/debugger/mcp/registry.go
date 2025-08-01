// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/mcp/mcpprompts"
	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/mcp/mcpresources"
	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/mcp/mcptools"
)

func RegisterAllTools(s *server.MCPServer) {
	s.AddTool(mcptools.NewFileCheckTool(), mcptools.HandleFileCheck)
	s.AddTool(mcptools.NewEndpointCheckTool(), mcptools.HandleEndpointCheck)
	s.AddTool(mcptools.NewLogCheckTool(), mcptools.HandleLogCheck)
}

func RegisterAllResources(s *server.MCPServer) {
	s.AddResource(mcpresources.NewLogResource(), mcpresources.HandleAgentLogFile)
}

func RegisterAllPrompts(s *server.MCPServer) {
	s.AddPrompt(mcpprompts.NewCloudwatchTroubleshootingPrompt(), mcpprompts.HandleCloudwatchTroubleshootingPrompt)
}
