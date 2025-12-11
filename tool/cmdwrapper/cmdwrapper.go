// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cmdwrapper

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type Flag struct {
	DefaultValue string
	Description  string
	IsBool       bool
}

// Make execCommand a variable that can be replaced in tests
var execCommand = exec.Command

// Make findAgentBinary a func variable to replace in tests
var findAgentBinary = func() (string, error) {
	// check in the current executable's directory first
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		currentDirPath := filepath.Join(execDir, paths.AgentBinaryName)
		if _, err := os.Stat(currentDirPath); err == nil {
			return currentDirPath, nil
		}
	}

	// fallback to the default path
	if _, err := os.Stat(paths.AgentBinaryPath); err == nil {
		return paths.AgentBinaryPath, nil
	}

	return "", fmt.Errorf("amazon-cloudwatch-agent binary not found at default path: %s", paths.AgentBinaryPath)
}

func CreateFlagSet(command string, flagConfigs map[string]Flag) (*flag.FlagSet, map[string]*string) {
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	flags := make(map[string]*string)
	for key, flagConfig := range flagConfigs {
		if flagConfig.IsBool {
			strPtr := new(string)
			*strPtr = flagConfig.DefaultValue
			fs.BoolFunc(key, flagConfig.Description, func(value string) error {
				if value == "" || value == "true" {
					*strPtr = "true"
				} else {
					*strPtr = "false"
				}
				return nil
			})
			flags[key] = strPtr
		} else {
			flags[key] = fs.String(key, flagConfig.DefaultValue, flagConfig.Description)
		}
	}
	return fs, flags
}

func ExecuteSubcommand(command string, flags map[string]*string) error {
	args := []string{command}

	for key, value := range flags {
		if *value != "" && *value != "false" {
			if *value == "true" {
				args = append(args, fmt.Sprintf("-%s", key))
			} else {
				args = append(args, fmt.Sprintf("-%s", key), *value)
			}
		}
	}

	agentPath, err := findAgentBinary()
	if err != nil {
		// Handle error appropriately
		return err
	}
	fmt.Printf("Executing %s with arguments: %v", paths.AgentBinaryPath, args)
	// Use execCommand instead of exec.Command directly
	cmd := execCommand(agentPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("e! %s process exited with non-zero status: %d", command, exitErr.ExitCode())
		}
		return fmt.Errorf("e! %s failed. Error: %v", command, err)
	}

	return nil
}

// HandleSubcommand processes subcommands with proper flag isolation
func HandleSubcommand(subcommands map[string]map[string]Flag, handlers map[string]func(map[string]*string) error) error {
	if len(os.Args) < 2 {
		return fmt.Errorf("no subcommand provided")
	}

	subcmd := os.Args[1]
	flagConfigs, exists := subcommands[subcmd]
	if !exists {
		return fmt.Errorf("unknown subcommand: %s", subcmd)
	}

	fs, flags := CreateFlagSet(subcmd, flagConfigs)
	fs.Parse(os.Args[2:])

	handler, exists := handlers[subcmd]
	if !exists {
		return fmt.Errorf("no handler for subcommand: %s", subcmd)
	}

	return handler(flags)
}
