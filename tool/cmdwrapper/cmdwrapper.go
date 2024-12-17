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

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type Flag struct {
	DefaultValue string
	Description  string
}

const delimiter = "-"

// Make execCommand a variable that can be replaced in tests
var execCommand = exec.Command

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

	// Use execCommand instead of exec.Command directly
	cmd := execCommand(paths.AgentBinaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			log.Panicf("E! Translation process exited with non-zero status: %d, err: %v",
				exitErr.ExitCode(), exitErr)
		}
		log.Panicf("E! Translation process failed. Error: %v", err)
		return err
	}

	return nil
}
