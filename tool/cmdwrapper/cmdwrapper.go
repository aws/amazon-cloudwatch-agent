// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cmdwrapper

import (
	"errors"
	"flag"
	"fmt"
	"log"
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
var findAgentBinary = func(path string) (string, error) {
	// check if the binary is at the normal path
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// if not, check in the current executable's directory
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get current executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)
	alternatePath := filepath.Join(execDir, paths.AgentBinaryName)

	// check again with alternate path
	if _, err := os.Stat(alternatePath); err == nil {
		return alternatePath, nil
	}

	return "", fmt.Errorf("amazon-cloudwatch-agent binary not found. expected in one of the following paths: %s or %s", path, execDir)
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

	log.Printf("Executing %s with arguments: %v", paths.AgentBinaryPath, args)

	agentPath, err := findAgentBinary(paths.AgentBinaryPath)
	if err != nil {
		// Handle error appropriately
		return err
	}
	// Use execCommand instead of exec.Command directly
	cmd := execCommand(agentPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			log.Fatalf("E! %s process exited with non-zero status: %d", command, exitErr.ExitCode())
		}
		log.Fatalf("E! %s failed. Error: %v", command, err)
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
