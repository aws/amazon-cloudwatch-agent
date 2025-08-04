// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mcptools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger"
)

func NewFileCheckTool() mcp.Tool {
	return mcp.NewTool("file_check",
		mcp.WithDescription(`Validates the existence and accessibility of critical CloudWatch agent files.

		This tool performs a comprehensive check of all key files required for the CloudWatch
		agent to function properly, including configuration files, log files, and runtime
		directories. It verifies file existence, readability permissions, and validates JSON
		configuration file syntax.

		The function examines both required files (essential for agent operation) and optional
		files, providing detailed status information for each. It automatically categorizes
		files by importance and health status to help identify critical issues.

		Usage: Use this tool to diagnose file-related issues when the CloudWatch agent fails
		to start or behaves unexpectedly, or to verify proper installation and configuration.

		Returns:
			FileCheckResponse: An object containing file status details, health summary,
			and JSON validation results for all checked files`))
}

type FileCheckResult struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Healthy     bool   `json:"healthy"`
}

// No arguments are required for this check. These parameters are here to match the interface.
// Errors are passed through NewToolResultError() and not through the error output. The error output is required for interface purposes.
func HandleFileCheck(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var outputBuffer bytes.Buffer
	hasValidJSON := debugger.IsConfigFilesPresentAndReadable(&outputBuffer, true) // Force compact format

	files := parseCompactOutput(outputBuffer.String())

	response := map[string]interface{}{
		"success":        true,
		"has_valid_json": hasValidJSON,
		"files":          files,
		"summary": map[string]interface{}{
			"total_files":      len(files),
			"healthy_files":    countHealthyFiles(files),
			"required_files":   countRequiredFiles(files),
			"missing_required": countMissingRequired(files),
		},
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error marshaling JSON: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

func parseCompactOutput(output string) []FileCheckResult {
	lines := strings.Split(output, "\n")
	re := regexp.MustCompile(`^([^:]+):\s+([^-]+)\s+-\s+(.+)$`)
	var files []FileCheckResult

	for _, line := range lines {
		if matches := re.FindStringSubmatch(strings.TrimSpace(line)); matches != nil {
			name := strings.TrimSpace(matches[1])
			status := strings.TrimSpace(matches[2])
			description := strings.TrimSpace(matches[3])

			files = append(files, FileCheckResult{
				Name:        name,
				Status:      status,
				Description: description,
				Required:    isRequired(name),
				Healthy:     isHealthy(status),
			})
		}
	}
	return files
}

func isRequired(name string) bool {
	requiredFiles := map[string]bool{
		"amazon-cloudwatch-agent.toml": true,
		"amazon-cloudwatch-agent.d":    true,
		"amazon-cloudwatch-agent.log":  true,
	}
	return requiredFiles[name]
}

func isHealthy(status string) bool {
	return strings.HasPrefix(status, "✓")
}

func countHealthyFiles(files []FileCheckResult) int {
	count := 0
	for _, f := range files {
		if f.Healthy {
			count++
		}
	}
	return count
}

func countRequiredFiles(files []FileCheckResult) int {
	count := 0
	for _, f := range files {
		if f.Required {
			count++
		}
	}
	return count
}

func countMissingRequired(files []FileCheckResult) int {
	count := 0
	for _, f := range files {
		if f.Required && !f.Healthy {
			count++
		}
	}
	return count
}
