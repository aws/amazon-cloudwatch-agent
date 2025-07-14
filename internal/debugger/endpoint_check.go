// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"fmt"
	"io"
)

type EndpointInfo struct {
	MetricsEndpoint string `json:"metrics_endpoint"`
	LogsEndpoint    string `json:"logs_endpoint"`
}

func CheckEndpoints(w io.Writer, config map[string]interface{}, ssm bool) (*EndpointInfo, error) {
	if config == nil {
		fmt.Fprintln(w, "No configuration available")
		return &EndpointInfo{
			MetricsEndpoint: "No configuration available",
			LogsEndpoint:    "No configuration available",
		}, nil
	}

	fmt.Fprintln(w, "\n=== Endpoint Configuration ===")

	var metricsEndpoint, logsEndpoint string
	if metrics, ok := config["metrics"].(map[string]interface{}); ok {
		if endpoint, ok := metrics["endpoint_override"].(string); ok {
			metricsEndpoint = endpoint
		} else {
			metricsEndpoint = "Default CloudWatch endpoint (no override)"
		}
	} else {
		metricsEndpoint = "No metrics configuration found"
	}

	if logs, ok := config["logs"].(map[string]interface{}); ok {
		if endpoint, ok := logs["endpoint_override"].(string); ok {
			logsEndpoint = endpoint
		} else {
			logsEndpoint = "Default CloudWatch Logs endpoint (no override)"
		}
	} else {
		logsEndpoint = "No logs configuration found"
	}

	if ssm {
		fmt.Fprintf(w, "Metrics: %s\n", metricsEndpoint)
		fmt.Fprintf(w, "Logs:    %s\n", logsEndpoint)
	} else {
		labelWidth := 15
		maxValueWidth := max(len(metricsEndpoint), len(logsEndpoint))
		maxValueWidth = max(maxValueWidth, 30)

		fmt.Fprintf(w, "┌%s┬%s┐\n",
			repeatChar('─', labelWidth+2),
			repeatChar('─', maxValueWidth+2))

		fmt.Fprintf(w, "│ %-*s │ %-*s │\n", labelWidth, "Metrics", maxValueWidth, metricsEndpoint)
		fmt.Fprintf(w, "│ %-*s │ %-*s │\n", labelWidth, "Logs", maxValueWidth, logsEndpoint)

		fmt.Fprintf(w, "└%s┴%s┘\n",
			repeatChar('─', labelWidth+2),
			repeatChar('─', maxValueWidth+2))
	}

	return &EndpointInfo{
		MetricsEndpoint: metricsEndpoint,
		LogsEndpoint:    logsEndpoint,
	}, nil
}
