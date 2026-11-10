// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import "testing"

func TestGetPrometheusLogLevel(t *testing.T) {
	tests := []struct {
		name          string
		debugEnv      string
		agentLogLevel string
		want          string
	}{
		{
			name: "default info level",
			want: "info",
		},
		{
			name:     "legacy DEBUG environment variable",
			debugEnv: "true",
			want:     "debug",
		},
		{
			name:          "agent debug log level",
			agentLogLevel: "DEBUG",
			want:          "debug",
		},
		{
			name:          "case insensitive agent log level",
			agentLogLevel: "debug",
			want:          "debug",
		},
		{
			name:          "agent info level",
			agentLogLevel: "INFO",
			want:          "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPrometheusLogLevel(tt.debugEnv, tt.agentLogLevel)
			if got != tt.want {
				t.Errorf("getPrometheusLogLevel() = %q, want %q", got, tt.want)
			}
		})
	}
}
