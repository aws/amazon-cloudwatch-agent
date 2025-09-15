// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cmdwrapper

import (
	"os"
	"os/exec"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

func TestCreateFlagSet(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		flagConfigs map[string]Flag
		want        map[string]string
	}{
		{
			name:    "basic flags",
			command: "test-command",
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
			fs, flags := CreateFlagSet(tt.command, tt.flagConfigs)

			if fs.Name() != tt.command {
				t.Errorf("Expected FlagSet name %s, got %s", tt.command, fs.Name())
			}

			for key, wantValue := range tt.want {
				if gotFlag, exists := flags[key]; !exists {
					t.Errorf("CreateFlagSet() missing key %s", key)
				} else if *gotFlag != wantValue {
					t.Errorf("CreateFlagSet() for key %s = %v, want %v", key, *gotFlag, wantValue)
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
	findAgentBinary = func() (string, error) {
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
	err := ExecuteSubcommand(command, flags)

	// Verify no error occurred
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the binary path
	if capturedPath != paths.AgentBinaryPath {
		t.Errorf("Expected binary path %s, got %s", paths.AgentBinaryPath, capturedPath)
	}

	// Verify first argument is the command
	if len(capturedArgs) == 0 || capturedArgs[0] != "fetch-config" {
		t.Errorf("Expected first argument to be fetch-config, got %v", capturedArgs)
		return
	}

	// Verify config flag is present with value
	if !containsSequence(capturedArgs, "-config", "config-value") {
		t.Errorf("Expected -config config-value sequence in %v", capturedArgs)
	}

	// Verify mode flag is present with value
	if !containsSequence(capturedArgs, "-mode", "mode-value") {
		t.Errorf("Expected -mode mode-value sequence in %v", capturedArgs)
	}

	// Verify expected number of arguments (command + 2 flags with values = 5)
	if len(capturedArgs) != 5 {
		t.Errorf("Expected 5 arguments, got %d: %v", len(capturedArgs), capturedArgs)
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
	findAgentBinary = func() (string, error) {
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
	err := ExecuteSubcommand(command, flags)

	// Verify no error occurred
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the first argument is the command
	if len(capturedArgs) == 0 || capturedArgs[0] != "config-wizard" {
		t.Errorf("Expected first argument to be config-wizard, got %v", capturedArgs)
		return
	}

	// Verify boolean true flag is present (without value)
	if !contains(capturedArgs, "-boolTrue") {
		t.Errorf("Expected -boolTrue flag to be present in %v", capturedArgs)
	}

	// Verify string flag is present with value
	if !containsSequence(capturedArgs, "-stringFlag", "test-value") {
		t.Errorf("Expected -stringFlag test-value sequence in %v", capturedArgs)
	}

	// Verify boolean false flag is NOT present
	if contains(capturedArgs, "-boolFalse") {
		t.Errorf("Expected -boolFalse flag to NOT be present in %v", capturedArgs)
	}

	// Verify empty string flag is NOT present
	if contains(capturedArgs, "-emptyString") {
		t.Errorf("Expected -emptyString flag to NOT be present in %v", capturedArgs)
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
			flagConfigs := map[string]Flag{
				"testBool": {
					DefaultValue: "false",
					Description:  "test boolean flag",
					IsBool:       true,
				},
			}

			fs, flags := CreateFlagSet("test", flagConfigs)
			fs.Parse(tt.args)

			if *flags["testBool"] != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, *flags["testBool"])
			}
		})
	}
}

func TestMixedFlags(t *testing.T) {
	flagConfigs := map[string]Flag{
		"boolFlag":   {DefaultValue: "false", Description: "boolean flag", IsBool: true},
		"stringFlag": {DefaultValue: "default", Description: "string flag"},
	}

	fs, flags := CreateFlagSet("test", flagConfigs)
	fs.Parse([]string{"-boolFlag", "-stringFlag=value"})

	if *flags["boolFlag"] != "true" {
		t.Errorf("Expected boolean flag to be true, got %s", *flags["boolFlag"])
	}

	if *flags["stringFlag"] != "value" {
		t.Errorf("Expected string flag to be 'value', got %s", *flags["stringFlag"])
	}
}

func TestHandleSubcommand(t *testing.T) {
	// Save original os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	subcommands := map[string]map[string]Flag{
		"test-cmd": {
			"flag1": {DefaultValue: "default", Description: "test flag"},
			"flag2": {DefaultValue: "false", Description: "bool flag", IsBool: true},
		},
	}

	var capturedFlags map[string]*string
	handlers := map[string]func(map[string]*string) error{
		"test-cmd": func(flags map[string]*string) error {
			capturedFlags = flags
			return nil
		},
	}

	// Test successful subcommand handling
	os.Args = []string{"program", "test-cmd", "-flag1=value1", "-flag2"}

	err := HandleSubcommand(subcommands, handlers)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if *capturedFlags["flag1"] != "value1" {
		t.Errorf("Expected flag1 to be 'value1', got %s", *capturedFlags["flag1"])
	}

	if *capturedFlags["flag2"] != "true" {
		t.Errorf("Expected flag2 to be 'true', got %s", *capturedFlags["flag2"])
	}
}
