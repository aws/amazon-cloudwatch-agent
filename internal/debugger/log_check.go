// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AgentLogConfig struct {
	LogGroupName    string `json:"log_group_name"`
	LogStreamName   string `json:"log_stream_name"`
	FilePath        string `json:"file_path"`
	RetentionDays   int    `json:"retention_in_days"`
	LogGroupClass   string `json:"log_group_class"`
	Timezone        string `json:"timezone,omitempty"`
	TimestampFormat string `json:"timestamp_format,omitempty"`
	Exists          bool   `json:"-"`
	Readable        bool   `json:"-"`
	Message         string `json:"-"`
}

func CheckLogs(config map[string]interface{}, ssm bool) ([]AgentLogConfig, error) {

	collectList, err := getCollectListFromConfig(config)
	if err != nil {
		fmt.Println("Error: Unable to find valid log collection configuration")
		return []AgentLogConfig{}, err
	}

	if len(collectList) == 0 {
		fmt.Println("No log files are configured")
		return []AgentLogConfig{}, nil
	}

	var logConfigs []AgentLogConfig

	for _, item := range collectList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		logConfig := parseLogConfig(itemMap)
		if logConfig.FilePath == "" {
			continue
		}

		matchedFiles := expandGlob(logConfig.FilePath)

		if len(matchedFiles) == 0 {
			logConfig.Exists = false
			logConfig.Readable = false
			logConfig.Message = fmt.Sprintf("No files match configured pattern %s", filepath.Base(logConfig.FilePath))
			logConfigs = append(logConfigs, logConfig)
		} else {
			for _, matchedFile := range matchedFiles {
				fileConfig := logConfig
				fileConfig.FilePath = matchedFile
				checkLogFileStatus(&fileConfig)
				logConfigs = append(logConfigs, fileConfig)
			}
		}
	}

	totalConfigs := len(logConfigs)
	accessibleConfigs := 0
	issueConfigs := 0

	for _, config := range logConfigs {
		if !config.Exists || !config.Readable {
			issueConfigs++
			AddConfigError(config.Message)
		} else {
			accessibleConfigs++
		}
	}

	fmt.Println("\n=== Log Configuration Summary ===")

	if ssm {
		fmt.Printf("Total Configurations: %d\n", totalConfigs)
		fmt.Printf("Accessible:           %d\n", accessibleConfigs)
		fmt.Printf("With Issues:          %d\n", issueConfigs)
	} else {
		labelWidth := 20
		valueWidth := 10

		fmt.Printf("┌%s┬%s┐\n",
			repeatChar('─', labelWidth+2),
			repeatChar('─', valueWidth+2))

		fmt.Printf("│ %-*s │ %-*d │\n", labelWidth, "Total Configurations", valueWidth, totalConfigs)
		fmt.Printf("│ %-*s │ %-*d │\n", labelWidth, "Accessible", valueWidth, accessibleConfigs)
		fmt.Printf("│ %-*s │ %-*d │\n", labelWidth, "With Issues", valueWidth, issueConfigs)

		fmt.Printf("└%s┴%s┘\n",
			repeatChar('─', labelWidth+2),
			repeatChar('─', valueWidth+2))
	}
	fmt.Println()

	return logConfigs, nil
}

func parseLogConfig(itemMap map[string]interface{}) AgentLogConfig {
	config := AgentLogConfig{}

	if filePath, ok := itemMap["file_path"].(string); ok {
		config.FilePath = filePath
	}
	if logGroup, ok := itemMap["log_group_name"].(string); ok {
		config.LogGroupName = logGroup
	}
	if logStream, ok := itemMap["log_stream_name"].(string); ok {
		config.LogStreamName = logStream
	}
	if retention, ok := itemMap["retention_in_days"].(float64); ok {
		config.RetentionDays = int(retention)
	}
	if class, ok := itemMap["log_group_class"].(string); ok {
		config.LogGroupClass = class
	}
	if timezone, ok := itemMap["timezone"].(string); ok {
		config.Timezone = timezone
	}
	if timestampFormat, ok := itemMap["timestamp_format"].(string); ok {
		config.TimestampFormat = timestampFormat
	}

	return config
}

func checkLogFileStatus(config *AgentLogConfig) {
	name := config.FilePath
	if _, err := os.Stat(config.FilePath); err != nil {
		config.Exists = false
		config.Readable = false
		config.Message = fmt.Sprintf("Configured log file %s does not exist.", name)
		return
	}

	config.Exists = true

	if file, err := os.Open(config.FilePath); err != nil {
		config.Readable = false
		config.Message = fmt.Sprintf("Agent does not have read permission for log file %s.", name)
	} else {
		file.Close()
		config.Readable = true
		// This doesn't get printed, but is stored for consistency.
		config.Message = fmt.Sprintf("Log file %s is accessible", name)
	}
}

func expandGlob(pattern string) []string {
	if strings.Contains(pattern, "**") {
		return expandRecursiveGlob(pattern)
	}

	if strings.ContainsAny(pattern, "*?") {
		matchedFiles, err := filepath.Glob(pattern)
		if err != nil {
			return []string{}
		}
		return matchedFiles
	}

	return []string{pattern}
}

func expandRecursiveGlob(pattern string) []string {
	var matches []string

	if strings.Contains(pattern, "**") {
		i := strings.Index(pattern, "**")
		basePath := pattern[:i]
		suffix := pattern[i+2:]

		if basePath == "" {
			basePath = "."
		}
		basePath = filepath.Clean(strings.TrimSuffix(basePath, "/"))
		suffix = strings.TrimPrefix(suffix, "/")

		err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() && suffix != "" {
				return nil
			}

			if suffix != "" {
				matched, _ := filepath.Match(suffix, filepath.Base(path))
				if matched {
					matches = append(matches, path)
				}
			} else if !info.IsDir() {
				matches = append(matches, path)
			}
			return nil
		})

		if err != nil {
			return []string{}
		}
	}

	return matches
}

func getCollectListFromConfig(config map[string]interface{}) ([]interface{}, error) {
	logs, ok := config["logs"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("logs configuration not found. Unable to parse log configuration")
	}

	logsCollected, ok := logs["logs_collected"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("logs_collected configuration not found. Unable to parse log configuration")
	}

	files, ok := logsCollected["files"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("files configuration not found. Unable to parse log configuration")
	}

	collectList, ok := files["collect_list"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("collect_list configuration not found. Unable to parse log configuration")
	}

	return collectList, nil
}
