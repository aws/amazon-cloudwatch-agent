// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/utils"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type FileStatus string

const (
	StatusPresent            FileStatus = "✓ Present"
	StatusMissing            FileStatus = "✗ Missing"
	StatusNoFile             FileStatus = "✗ No file found"
	StatusInvalidJSONFormat  FileStatus = "! Invalid JSON format"
	StatusPresentNotReadable FileStatus = "! Present but not readable"
)

type ConfigFile struct {
	Path        string
	Description string
	Required    bool
	Purpose     string
	MissingMsg  string
}

func IsConfigFilesPresentAndReadable(w io.Writer, compact bool) bool {
	fmt.Fprintln(w, "\n=== Configuration Files ===")

	configFiles := getConfigFiles()

	if compact {
		printConfigFilesCompact(w, configFiles)
	} else {
		printConfigFilesTable(w, configFiles)
	}

	jsonConfigStatus := checkFileStatus(paths.ConfigDirPath)
	return jsonConfigStatus == StatusPresent
}

func printConfigFilesCompact(w io.Writer, configFiles []ConfigFile) {
	// Calculate max display name width for alignment
	maxNameWidth := 0
	for _, file := range configFiles {
		displayName := getDisplayName(file.Path)
		maxNameWidth = max(maxNameWidth, len(displayName)+1) // +1 for colon
	}

	errorCollector := GetErrorCollector()
	for _, file := range configFiles {
		status := checkFileStatus(file.Path)
		displayName := getDisplayName(file.Path)
		fmt.Fprintf(w, "%-*s %s - %s\n", maxNameWidth, displayName+":", status, file.Description)
		handleFileStatus(file, status, errorCollector)
	}
}

func printConfigFilesTable(w io.Writer, configFiles []ConfigFile) {
	fileNameWidth := 25
	statusWidth := 20
	for _, file := range configFiles {
		displayName := getDisplayName(file.Path)
		fileNameWidth = max(fileNameWidth, len(displayName))
		status := checkFileStatus(file.Path)
		statusWidth = max(statusWidth, len(string(status)))
	}

	fmt.Fprintf(w, "┌%s┬%s┬%s┐\n",
		utils.RepeatChar('─', fileNameWidth+2),
		utils.RepeatChar('─', statusWidth+2),
		utils.RepeatChar('─', 50))

	fmt.Fprintf(w, "│ %-*s │ %-*s │ %-48s │\n", fileNameWidth, "File", statusWidth, "Status", "Description")

	fmt.Fprintf(w, "├%s┼%s┼%s┤\n",
		utils.RepeatChar('─', fileNameWidth+2),
		utils.RepeatChar('─', statusWidth+2),
		utils.RepeatChar('─', 50))

	errorCollector := GetErrorCollector()
	for _, file := range configFiles {
		status := checkFileStatus(file.Path)
		displayName := getDisplayName(file.Path)
		description := file.Description
		if len(description) > 48 {
			description = description[:45] + "..."
		}

		fmt.Fprintf(w, "│ %-*s │ %-*s │ %-48s │\n", fileNameWidth, displayName, statusWidth, status, description)
		handleFileStatus(file, status, errorCollector)
	}

	fmt.Fprintf(w, "└%s┴%s┴%s┘\n",
		utils.RepeatChar('─', fileNameWidth+2),
		utils.RepeatChar('─', statusWidth+2),
		utils.RepeatChar('─', 50))
}

func handleFileStatus(file ConfigFile, status FileStatus, errorCollector *ErrorCollector) {
	if status != StatusPresent {
		message := fmt.Sprintf("%s: %s - %s", getDisplayName(file.Path), file.MissingMsg, file.Purpose)

		// For specific error types, add more context
		switch status {
		case StatusInvalidJSONFormat:
			message = fmt.Sprintf("%s: Invalid JSON format - %s", getDisplayName(file.Path), file.Purpose)
		}

		if file.Required {
			errorCollector.AddError(message)
		} else {
			errorCollector.AddWarning(message)
		}
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
		return FileStatus("Error checking directory: " + err.Error())
	}

	validJsonFiles := 0
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".yaml") {
			filePath := filepath.Join(path, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			var js map[string]interface{}
			if err := json.Unmarshal(content, &js); err == nil {
				validJsonFiles++
			}
		}
	}

	if validJsonFiles == 0 {
		return StatusNoFile
	}

	return StatusPresent
}

func getConfigFiles() []ConfigFile {
	baseFiles := []ConfigFile{
		{
			Path:        paths.TomlConfigPath,
			Description: "Main TOML configuration file",
			Required:    true,
			Purpose:     "Defines metrics, logs, and traces collection settings",
			MissingMsg:  "Agent cannot start without this configuration file. This could point to a translator issue.",
		},
		{
			Path:        paths.YamlConfigPath,
			Description: "YAML configuration file",
			Required:    false,
			Purpose:     "Alternative configuration format for OpenTelemetry components",
			MissingMsg:  "Only needed if using YAML-based configuration instead of JSON/TOML",
		},
		{
			Path:        paths.CommonConfigPath,
			Description: "Common configuration file",
			Required:    false,
			Purpose:     "Configures AWS credentials, proxy, SSL, and IMDS settings for agent communication",
			MissingMsg:  "Only needed if overriding default AWS credentials or configuring proxy/SSL settings OR with on-prem instances.",
		},
		{
			Path:        paths.EnvConfigPath,
			Description: "Environment configuration file",
			Required:    false,
			Purpose:     "Environment-specific overrides and settings",
			MissingMsg:  "Only required if using environment-specific configuration overrides",
		},
		{
			Path:        paths.AgentLogFilePath,
			Description: "Agent's log file",
			Required:    true,
			Purpose:     "Contains agent runtime logs for troubleshooting",
			MissingMsg:  "Log file should exist after agent starts - check if agent is running",
		},
		{
			Path:        paths.ConfigDirPath,
			Description: "JSON configuration directory",
			Required:    true,
			Purpose:     "Contains JSON format configuration files for agent operation",
			MissingMsg:  "Primary configuration method, agent needs this to function",
		},
	}

	// Add individual files from .d directory
	if entries, err := os.ReadDir(paths.ConfigDirPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".yaml") {
				baseFiles = append(baseFiles, ConfigFile{
					Path:        filepath.Join(paths.ConfigDirPath, entry.Name()),
					Description: "Configuration file in .d directory",
					Required:    false,
					Purpose:     "Individual configuration file processed by agent",
					MissingMsg:  "Configuration file in directory",
				})
			}
		}
	}

	return baseFiles
}

func getDisplayName(path string) string {
	if strings.HasSuffix(path, ".d") {
		return "amazon-cloudwatch-agent.d"
	}
	return filepath.Base(path)
}
