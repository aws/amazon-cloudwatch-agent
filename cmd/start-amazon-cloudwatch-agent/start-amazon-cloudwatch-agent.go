// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"syscall"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/constants"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

func translateConfig() error {
	args := []string{"--output", paths.TomlConfigPath, "--mode", "auto"}
	if envconfig.IsRunningInContainer() {
		args = append(args, "--input-dir", paths.CONFIG_DIR_IN_CONTAINER)
	} else {
		args = append(args, "--input", paths.JsonConfigPath, "--input-dir", paths.JsonDirPath, "--config", paths.CommonConfigPath)
	}
	cmd := exec.Command(paths.TranslatorBinaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			switch {
			case status.Exited():
				log.Printf("I! Return exit error: exit code=%d\n", status.ExitStatus())

				if status.ExitStatus() == constants.ExitCodeNoJSONFile {
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

	if !envconfig.IsRunningInContainer() {
		writer = &lumberjack.Logger{
			Filename:   paths.AgentLogFilePath,
			MaxSize:    100, //MB
			MaxBackups: 5,   //backup files
			MaxAge:     7,   //days
			Compress:   true,
		}

		log.SetOutput(writer)
	}

	if err := translateConfig(); err != nil {
		log.Fatalf("E! Cannot translate JSON, ERROR is %v \n", err)
	}
	log.Printf("I! Config has been translated into TOML %s \n", paths.TomlConfigPath)
	printFileContents(paths.TomlConfigPath)
	log.Printf("I! Config has been translated into YAML %s \n", paths.YamlConfigPath)
	printFileContents(paths.YamlConfigPath)

	if err := startAgent(writer); err != nil {
		log.Printf("E! Error when starting Agent, Error is %v \n", err)
		os.Exit(1)
	}
}

func printFileContents(path string) {
	file, err := os.Open(path)
	if err != nil {
		// YAML file may or may not exist and that is okay.
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("E! Error when printing file(%s) contents, Error is %v \n", path, err)
		}
		return
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Printf("E! Error when closing file, Error is %v \n", err)
		}
	}()

	b, err := io.ReadAll(file)
	if err != nil {
		log.Printf("E! Error when reading file(%s), Error is %v \n", path, err)
	}
	log.Printf("D! config %v", string(b))
}
