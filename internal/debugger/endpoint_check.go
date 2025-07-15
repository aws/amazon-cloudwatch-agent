// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"fmt"
	"io"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/utils"
)

type EndpointInfo struct {
	MetricsEndpoint string `json:"metrics_endpoint"`
	LogsEndpoint    string `json:"logs_endpoint"`
}

func CheckEndpoints(w io.Writer, config map[string]interface{}, compact bool) (*EndpointInfo, error) {
	if config == nil {
		fmt.Fprintln(w, "No configuration available")
		return &EndpointInfo{
			MetricsEndpoint: "No configuration available",
			LogsEndpoint:    "No configuration available",
		}, nil
	}

	fmt.Fprintln(w, "\n=== Endpoint Configuration ===")

	metricsEndpoint := getEndpoint(config, "metrics", "Default CloudWatch endpoint (no override)", "No metrics configuration found")
	logsEndpoint := getEndpoint(config, "logs", "Default CloudWatch Logs endpoint (no override)", "No logs configuration found")

	if compact {
		printSSMFormat(w, metricsEndpoint, logsEndpoint)
	} else {
		printTableFormat(w, metricsEndpoint, logsEndpoint)
	}

	return &EndpointInfo{
		MetricsEndpoint: metricsEndpoint,
		LogsEndpoint:    logsEndpoint,
	}, nil
}

func getEndpoint(config map[string]interface{}, section, defaultMsg, notFoundMsg string) string {
	sectionConfig, ok := config[section].(map[string]interface{})
	if !ok {
		return notFoundMsg
	}
	if endpoint, ok := sectionConfig["endpoint_override"].(string); ok {
		return endpoint
	}
	return defaultMsg
}

func printSSMFormat(w io.Writer, metricsEndpoint, logsEndpoint string) {
	fmt.Fprintf(w, "Metrics: %s\n", metricsEndpoint)
	fmt.Fprintf(w, "Logs:    %s\n", logsEndpoint)
}

func printTableFormat(w io.Writer, metricsEndpoint, logsEndpoint string) {
	labelWidth := 15
	maxValueWidth := max(len(metricsEndpoint), len(logsEndpoint))
	maxValueWidth = max(maxValueWidth, 30)

	fmt.Fprintf(w, "┌%s┬%s┐\n",
		utils.RepeatChar('─', labelWidth+2),
		utils.RepeatChar('─', maxValueWidth+2))

	fmt.Fprintf(w, "│ %-*s │ %-*s │\n", labelWidth, "Metrics", maxValueWidth, metricsEndpoint)
	fmt.Fprintf(w, "│ %-*s │ %-*s │\n", labelWidth, "Logs", maxValueWidth, logsEndpoint)

	fmt.Fprintf(w, "└%s┴%s┘\n",
		utils.RepeatChar('─', labelWidth+2),
		utils.RepeatChar('─', maxValueWidth+2))
}
