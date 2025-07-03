// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cmdwrapper

import (
	"flag"
	"os"
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
	originalFindAgentBinary := findAgentBinary
	defer func() {
		execCommand = originalExecCommand
		findAgentBinary = originalFindAgentBinary
	}()

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
	// Mock findAgentBinary to always return the agent binary path without checking if it exists
	findAgentBinary = func(_ string) (string, error) {
		return paths.AgentBinaryPath, nil
	}

	// Test data
	command := "fetch-config"
	configValue := "config-value"
	modeValue := "mode-value"
	flags := map[string]*string{
		"config": &configValue,
		"mode":   &modeValue,
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

func TestExecuteAgentCommand_BooleanFlags(t *testing.T) {
	// Save the original execCommand and restore it after the test
	originalExecCommand := execCommand
	originalFindAgentBinary := findAgentBinary
	defer func() {
		execCommand = originalExecCommand
		findAgentBinary = originalFindAgentBinary
	}()

	var capturedArgs []string

	// Mock execCommand
	execCommand = func(_ string, args ...string) *exec.Cmd {
		capturedArgs = args
		cmd := exec.Command("echo", "1")
		return cmd
	}
	findAgentBinary = func(_ string) (string, error) {
		return paths.AgentBinaryPath, nil
	}

	// Test data with boolean and string flags
	command := "config-wizard"
	trueValue := "true"
	falseValue := "false"
	stringValue := "test-value"
	emptyValue := ""
	flags := map[string]*string{
		"boolTrue":    &trueValue,
		"boolFalse":   &falseValue,
		"stringFlag":  &stringValue,
		"emptyString": &emptyValue,
	}

	// Execute the function
	err := ExecuteAgentCommand(command, flags)

	// Verify no error occurred
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the first argument is the command
	if len(capturedArgs) == 0 || capturedArgs[0] != "-config-wizard" {
		t.Errorf("Expected first argument to be -config-wizard, got %v", capturedArgs)
		return
	}

	// Verify boolean true flag is present (without value)
	if !contains(capturedArgs, "-config-wizard-boolTrue") {
		t.Errorf("Expected -config-wizard-boolTrue flag to be present in %v", capturedArgs)
	}

	// Verify string flag is present with value
	if !containsSequence(capturedArgs, "-config-wizard-stringFlag", "test-value") {
		t.Errorf("Expected -config-wizard-stringFlag test-value sequence in %v", capturedArgs)
	}

	// Verify boolean false flag is NOT present
	if contains(capturedArgs, "-config-wizard-boolFalse") {
		t.Errorf("Expected -config-wizard-boolFalse flag to NOT be present in %v", capturedArgs)
	}

	// Verify empty string flag is NOT present
	if contains(capturedArgs, "-config-wizard-emptyString") {
		t.Errorf("Expected -config-wizard-emptyString flag to NOT be present in %v", capturedArgs)
	}
}

// Helper function to check if slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// helper to check if slice contains a sequence of values
func containsSequence(slice []string, first, second string) bool {
	for i := 0; i < len(slice)-1; i++ {
		if slice[i] == first && slice[i+1] == second {
			return true
		}
	}
	return false
}

func TestBooleanFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "boolean flag without value",
			args:     []string{"-testBool"},
			expected: "true",
		},
		{
			name:     "boolean flag with true value",
			args:     []string{"-testBool=true"},
			expected: "true",
		},
		{
			name:     "boolean flag with false value",
			args:     []string{"-testBool=false"},
			expected: "false",
		},
		{
			name:     "boolean flag not provided",
			args:     []string{},
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("", flag.ExitOnError)

			flagConfigs := map[string]Flag{
				"testBool": {
					DefaultValue: "false",
					Description:  "test boolean flag",
					IsBool:       true,
				},
			}

			flags := AddFlags("", flagConfigs)

			oldArgs := os.Args
			os.Args = append([]string{"test"}, tt.args...)
			defer func() { os.Args = oldArgs }()

			flag.Parse()

			if *flags["testBool"] != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, *flags["testBool"])
			}
		})
	}
}

func TestMixedFlags(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("", flag.ExitOnError)

	flagConfigs := map[string]Flag{
		"boolFlag":   {DefaultValue: "false", Description: "boolean flag", IsBool: true},
		"stringFlag": {DefaultValue: "default", Description: "string flag"},
	}

	flags := AddFlags("", flagConfigs)

	oldArgs := os.Args
	os.Args = []string{"test", "-boolFlag", "-stringFlag=value"}
	defer func() { os.Args = oldArgs }()

	flag.Parse()

	if *flags["boolFlag"] != "true" {
		t.Errorf("Expected boolean flag to be true, got %s", *flags["boolFlag"])
	}

	if *flags["stringFlag"] != "value" {
		t.Errorf("Expected string flag to be 'value', got %s", *flags["stringFlag"])
	}
}
