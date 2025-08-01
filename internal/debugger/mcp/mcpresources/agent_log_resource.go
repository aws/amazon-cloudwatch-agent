package mcpresources

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/mark3labs/mcp-go/mcp"
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
