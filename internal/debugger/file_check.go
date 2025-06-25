// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type FileStatus string

const (
	StatusPresent            FileStatus = "✓ Present"
	StatusMissing            FileStatus = "✗ Missing"
	StatusNoFile             FileStatus = "✗ No file found"
	StatusMultipleFiles      FileStatus = "! Multiple files found"
	StatusNotReadable        FileStatus = "! File present but not readable"
	StatusInvalidJSON        FileStatus = "! Existent but invalid JSON"
	StatusInvalidJSONFormat  FileStatus = "! Invalid JSON format"
	StatusPresentNotReadable FileStatus = "! Present but not readable"
)

type ConfigFile struct {
	Path        string
	Description string
	Required    bool
}

// Checks for existence and readability of key configuration files
func CheckConfigFiles() {
	log.Println("Checking Configuration Files:")

	configFiles := getConfigFiles()
	for _, file := range configFiles {
		status := checkFileStatus(file.Path)
		displayName := getDisplayName(file.Path)
		log.Printf("%-20s [%s] - %s", displayName, status, file.Description)
	}
}

func checkFileStatus(path string) FileStatus {
	if strings.HasSuffix(path, ".d") {
		return checkDirectoryStatus(path)
	}
	return checkFileContentStatus(path)
}

func checkFileContentStatus(path string) FileStatus {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return StatusMissing
		}
		return FileStatus("? Error checking file: " + err.Error())
	}

	if strings.HasSuffix(path, ".log") {
		return StatusPresent
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return StatusPresentNotReadable
	}

	if strings.HasSuffix(path, ".json") {
		var js map[string]interface{}
		if err := json.Unmarshal(content, &js); err != nil {
			return StatusInvalidJSONFormat
		}
	}

	return StatusPresent
}

func checkDirectoryStatus(path string) FileStatus {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StatusMissing
		}
		return FileStatus("? Error checking directory: " + err.Error())
	}

	if len(entries) == 0 {
		return StatusNoFile
	}

	if len(entries) > 1 {
		return StatusMultipleFiles
	}

	fileName := entries[0].Name()
	content, err := os.ReadFile(filepath.Join(path, fileName))
	if err != nil {
		return StatusNotReadable
	}

	var js map[string]interface{}
	if err := json.Unmarshal(content, &js); err != nil {
		return StatusInvalidJSON
	}

	return StatusPresent
}

func getConfigFiles() []ConfigFile {
	return []ConfigFile{
		{Path: paths.TomlConfigPath, Description: "Main TOML configuration file", Required: true},
		{Path: paths.ConfigDirPath, Description: "JSON configuration file", Required: true},
		{Path: paths.YamlConfigPath, Description: "YAML configuration file", Required: true},
		{Path: paths.CommonConfigPath, Description: "Common configuration file", Required: false},
		{Path: filepath.Join(paths.AgentDir, "/etc/log-config.json"), Description: "Log configuration file", Required: false},
		{Path: paths.EnvConfigPath, Description: "Environment configuration file", Required: false},
		{Path: paths.AgentLogFilePath, Description: "Agent's log file", Required: true},
	}
}

func getDisplayName(path string) string {
	if strings.HasSuffix(path, ".d") {
		return "amazon-cloudwatch-agent.d"
	}
	return filepath.Base(path)
}
