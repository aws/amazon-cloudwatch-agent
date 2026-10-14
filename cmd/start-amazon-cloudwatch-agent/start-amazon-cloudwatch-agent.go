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
	"path/filepath"
	"syscall"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/constants"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

// isTranslationStale reports whether the config-translator subprocess chain
// must run. It returns false (fresh) when both always-produced outputs
// (paths.TomlConfigPath and paths.EnvConfigPath) exist and neither has any
// config source that is strictly newer than itself, so that an automatic
// service start after a reboot without config changes can skip the translation
// entirely.
//
// The comparison uses the older of the two output mtimes as the threshold:
// an input that post-dates any output signals a stale translation.
//
// The Configs directory is walked recursively to mirror the depth used by the
// translator itself (filepath.Walk in cmdutil.GenerateMergedJsonConfigMap).
// An unreadable Configs directory is silently treated as empty.
//
// Known limitations:
//   - Config files or the agent binary restored from backup with preserved
//     timestamps (older than the outputs) are treated as fresh even if their
//     contents changed. Re-translation can be forced by removing the TOML.
//
// isTranslationStale is never called in container mode; containers always run
// the full translation.
func isTranslationStale() bool {
	// Both outputs are always produced by a successful translation.
	// If either is absent, the previous translation was incomplete.
	tomlInfo, err := os.Stat(paths.TomlConfigPath)
	if err != nil {
		return true
	}
	envConfigInfo, err := os.Stat(paths.EnvConfigPath)
	if err != nil {
		return true
	}

	// Use the older of the two outputs as the comparison threshold so that
	// an input newer than the earliest output triggers re-translation.
	outMod := tomlInfo.ModTime()
	if envConfigInfo.ModTime().Before(outMod) {
		outMod = envConfigInfo.ModTime()
	}

	// Primary input files.
	inputPaths := []string{
		paths.JsonConfigPath,
		paths.CommonConfigPath,
		paths.AgentBinaryPath, // binary upgrade triggers retranslation
	}

	// Walk the Configs directory recursively (mirrors the translator's own
	// filepath.Walk over the same directory). An unreadable directory is
	// silently treated as empty.
	_ = filepath.WalkDir(paths.ConfigDirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		inputPaths = append(inputPaths, path)
		return nil
	})

	for _, input := range inputPaths {
		info, err := os.Stat(input)
		if err != nil {
			// Input does not exist; skip it.
			continue
		}
		if info.ModTime().After(outMod) {
			return true
		}
	}

	return false
}

func translateConfig() error {
	if !envconfig.IsRunningInContainer() && !isTranslationStale() {
		log.Printf("I! Config translation skipped: TOML is up-to-date\n")
		return nil
	}

	args := []string{"--output", paths.TomlConfigPath, "--mode", "auto"}
	if envconfig.IsRunningInContainer() {
		args = append(args, "--input-dir", paths.CONFIG_DIR_IN_CONTAINER)
	} else {
		args = append(args, "--input", paths.JsonConfigPath, "--input-dir", paths.ConfigDirPath, "--config", paths.CommonConfigPath)
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
