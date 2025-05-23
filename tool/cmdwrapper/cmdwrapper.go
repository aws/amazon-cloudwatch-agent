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
	"runtime"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type Flag struct {
	DefaultValue string
	Description  string
}

const delimiter = "-"

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
	alternatePath := filepath.Join(execDir, "amazon-cloudwatch-agent")
	if runtime.GOOS == "windows" {
		alternatePath += ".exe"
	}

	// check again with alternate path
	if _, err := os.Stat(alternatePath); err == nil {
		return alternatePath, nil
	}

	return "", fmt.Errorf("amazon-cloudwatch-agent binary cannot be found")
}

func AddFlags(prefix string, flagConfigs map[string]Flag) map[string]*string {
	flags := make(map[string]*string)
	for key, flagConfig := range flagConfigs {
		flagName := key
		if prefix != "" {
			flagName = prefix + delimiter + flagName
		}
		flags[key] = flag.String(flagName, flagConfig.DefaultValue, flagConfig.Description)
	}
	return flags
}

func ExecuteAgentCommand(command string, flags map[string]*string) error {
	args := []string{fmt.Sprintf("-%s", command)}

	for key, value := range flags {
		if *value != "" {
			args = append(args, fmt.Sprintf("-%s%s%s", command, delimiter, key), *value)
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
			log.Panicf("E! %s process exited with non-zero status: %d, err: %v", command, exitErr.ExitCode(), exitErr)
		}
		log.Panicf("E! %s failed. Error: %v", command, err)
		return err
	}

	return nil
}
