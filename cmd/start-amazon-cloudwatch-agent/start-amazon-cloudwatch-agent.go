// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	COMMON_CONFIG = "common-config.toml"
	JSON          = "amazon-cloudwatch-agent.json"
	TOML          = "amazon-cloudwatch-agent.toml"
	ENV           = "env-config.json"

	AGENT_LOG_FILE = "amazon-cloudwatch-agent.log"

	//TODO this CONFIG_DIR_IN_CONTAINE should change to something indicate dir, keep it for now to avoid break testing
	CONFIG_DIR_IN_CONTAINE = "/etc/cwagentconfig"
)

var (
	jsonConfigPath   string
	jsonDirPath      string
	envConfigPath    string
	tomlConfigPath   string
	commonConfigPath string

	agentLogFilePath string

	translatorBinaryPath string
	agentBinaryPath      string
)

// We use an environment variable here because we need this condition before the translator reads agent config json file.
var runInContainer = os.Getenv(config.RUN_IN_CONTAINER)

func translateConfig() error {
	args := []string{"--output", tomlConfigPath, "--mode", "auto"}
	if runInContainer == config.RUN_IN_CONTAINER_TRUE {
		args = append(args, "--input-dir", CONFIG_DIR_IN_CONTAINE)
	} else {
		args = append(args, "--input", jsonConfigPath, "--input-dir", jsonDirPath, "--config", commonConfigPath)
	}
	cmd := exec.Command(translatorBinaryPath, args...)

	stdoutStderr, err := cmd.CombinedOutput()
	log.Printf("I! %s \n", stdoutStderr)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			switch {
			case status.Exited():
				log.Printf("I! Return exit error: exit code=%d\n", status.ExitStatus())

				if status.ExitStatus() == config.ERR_CODE_NOJSONFILE {
					log.Printf("I! No json config files found, please provide config, exit now\n")
					os.Exit(0)
				}
			}
		} else {
			log.Printf("Return other error: %s\n", err)
		}
	}

	return err
}

func main() {
	var writer io.WriteCloser

	if runInContainer != config.RUN_IN_CONTAINER_TRUE {
		writer = &lumberjack.Logger{
			Filename:   agentLogFilePath,
			MaxSize:    100, //MB
			MaxBackups: 5,   //backup files
			MaxAge:     7,   //days
			Compress:   true,
		}

		log.SetOutput(writer)
	}

	if err := translateConfig(); err != nil {
		log.Fatalf("E! Cannot translate JSON config into TOML, ERROR is %v \n", err)
	}
	log.Printf("I! Config has been translated into TOML %s \n", tomlConfigPath)
	printFileContents(tomlConfigPath)

	if err := startAgent(writer); err != nil {
		log.Printf("E! Error when starting Agent, Error is %v \n", err)
		os.Exit(1)
	}
}

func printFileContents(path string) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("E! Error when printing file(%s) contents, Error is %v \n", path, err)
		os.Exit(1)
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Printf("E! Error when closing file,  Error is %v \n", err)
		}
	}()

	b, err := io.ReadAll(file)
	if err != nil {
		log.Printf("E! Error when reading file(%s), Error is %v \n", path, err)
	}
	log.Printf("D! toml config %v", string(b))
}
