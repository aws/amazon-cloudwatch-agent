// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cmdwrapper

import (
	"flag"
	"os/exec"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

func TestAddFlags(t *testing.T) {
	// Reset the flag package to avoid conflicts
	flag.CommandLine = flag.NewFlagSet("", flag.ExitOnError)

	tests := []struct {
		name        string
		prefix      string
		flagConfigs map[string]Flag
		want        map[string]string // Expected default values
	}{
		{
			name:   "no prefix",
			prefix: "",
			flagConfigs: map[string]Flag{
				"test": {
					DefaultValue: "default",
					Description:  "test description",
				},
			},
			want: map[string]string{
				"test": "default",
			},
		},
		{
			name:   "with prefix",
			prefix: "prefix",
			flagConfigs: map[string]Flag{
				"test": {
					DefaultValue: "default",
					Description:  "test description",
				},
			},
			want: map[string]string{
				"test": "default",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddFlags(tt.prefix, tt.flagConfigs)

			// Verify the returned map has the correct keys
			if len(got) != len(tt.want) {
				t.Errorf("AddFlags() returned map of size %d, want %d", len(got), len(tt.want))
			}

			// Verify default values
			for key, wantValue := range tt.want {
				if gotFlag, exists := got[key]; !exists {
					t.Errorf("AddFlags() missing key %s", key)
				} else if *gotFlag != wantValue {
					t.Errorf("AddFlags() for key %s = %v, want %v", key, *gotFlag, wantValue)
				}
			}
		})
	}
}

func TestExecuteAgentCommand_HappyPath(t *testing.T) {
	// Save the original execCommand and restore it after the test
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	var capturedPath string
	var capturedArgs []string

	// Mock execCommand
	execCommand = func(path string, args ...string) *exec.Cmd {
		capturedPath = path
		capturedArgs = args

		// Use "echo" as a no-op command that will succeed
		cmd := exec.Command("echo", "1")
		return cmd
	}

	// Test data
	command := "fetch-config"
	flags := map[string]*string{
		"config": stringPtr("config-value"),
		"mode":   stringPtr("mode-value"),
	}

	// Execute the function
	err := ExecuteAgentCommand(command, flags)

	// Verify no error occurred
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the binary path
	if capturedPath != paths.AgentBinaryPath {
		t.Errorf("Expected binary path %s, got %s", paths.AgentBinaryPath, capturedPath)
	}

	// Expected arguments
	expectedArgs := []string{
		"-fetch-config",
		"-fetch-config-config", "config-value",
		"-fetch-config-mode", "mode-value",
	}

	// Verify arguments length
	if len(capturedArgs) != len(expectedArgs) {
		t.Errorf("Expected %d arguments, got %d", len(expectedArgs), len(capturedArgs))
	}

	// Verify each argument
	for i, expected := range expectedArgs {
		if capturedArgs[i] != expected {
			t.Errorf("Argument %d: expected %s, got %s", i, expected, capturedArgs[i])
		}
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
