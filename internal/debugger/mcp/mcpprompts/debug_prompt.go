// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mcpprompts

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewCloudwatchTroubleshootingPrompt() mcp.Prompt {
	return mcp.NewPrompt("cloudwatch_troubleshooting",
		mcp.WithPromptDescription("Provides guidance for troubleshooting CloudWatch agent issues"),
		mcp.WithArgument("issue_description",
			mcp.ArgumentDescription("Description of the issue you're experiencing with the CloudWatch agent"),
		),
	)
}

func HandleCloudwatchTroubleshootingPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := request.Params.Arguments
	var issueDescription string
	if args != nil {
		if desc, ok := args["issue_description"]; ok && desc != "" {
			issueDescription = fmt.Sprintf("\n\nSpecific issue: %s", desc)
		}
	}

	promptContent := fmt.Sprintf(`You are helping troubleshoot AWS CloudWatch agent issues. When debugging CloudWatch agent problems, you should:

	1. **Always run the cloudwatch_debugger tools first.
	- These tools will do the logic and provide you with diagnostic information. It doesn't hurt to run all of them.

	2. **Always check the CloudWatch agent log file first**: /opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log
	- This log contains startup messages, configuration parsing errors, metric collection issues, and other diagnostic information
	- Look for ERROR, WARN, or FATAL level messages
	- Check timestamps to correlate with when issues started occurring

	3. **Common troubleshooting steps**:
	- Verify the agent is running: sudo systemctl status amazon-cloudwatch-agent
	- Check configuration syntax and permissions
	- Validate IAM permissions for CloudWatch and EC2 access
	- Ensure network connectivity to CloudWatch endpoints
	- Review metric filters and log group configurations

	4. **Key log patterns to look for**:
	- All error logs contain '!E' in them so you can filter by those
	- Configuration parsing errors
	- Permission denied errors  
	- Network connectivity issues
	- Metric collection failures
	- Log file access problems%s

	Start your troubleshooting by examining the log file and identifying any error messages or warnings that might indicate the root cause.`, issueDescription)

	return mcp.NewGetPromptResult(
		"CloudWatch agent troubleshooting guidance",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(promptContent),
			),
		},
	), nil
}
