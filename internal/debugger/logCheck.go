// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// contains the results of checking log files
type LogCheckResult struct {
	Success bool
	Message string
	Files   []LogFileStatus
}

// represents the status of a log file
type LogFileStatus struct {
	Path     string
	Exists   bool
	Readable bool
	Message  string
}

func CheckLogs(config map[string]interface{}) LogCheckResult {
	log.Println("Running CheckLogs...")
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panicked with error: %v", r)
			return
		}
	}()

	collectList, err := getCollectListFromConfig(config)
	if err != nil {
		log.Println("Error: Unable to find valid log collection configuration")
		return LogCheckResult{
			Success: false,
			Message: "Unable to find valid log collection configuration: " + err.Error(),
		}
	}

	if len(collectList) == 0 {
		log.Println("Nothing in collectList")
		return LogCheckResult{
			Success: true,
			Message: "No log files configured",
		}
	}

	result := LogCheckResult{
		Success: true,
		Message: "Log check completed",
		Files:   []LogFileStatus{},
	}

	for _, item := range collectList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		filePath, ok := itemMap["file_path"].(string)
		if !ok {
			continue
		}

		fileStatus := checkLogPermissions(filePath)
		result.Files = append(result.Files, fileStatus)

		// If any file is not readable, mark the overall check as unsuccessful
		if !fileStatus.Exists || !fileStatus.Readable {
			result.Success = false
		}
	}

	return result
}

func checkLogPermissions(filePath string) LogFileStatus {
	result := LogFileStatus{
		Path:     filePath,
		Exists:   false,
		Readable: false,
	}

	name := filepath.Base(filePath)
	if _, err := os.Stat(filePath); err != nil {
		msg := fmt.Sprintf("Configured log file %s does not exist.", name)
		log.Println(msg)
		result.Message = msg
		return result
	}

	result.Exists = true

	if file, err := os.Open(filePath); err != nil {
		msg := fmt.Sprintf("Agent does not have read permission for log file %s", name)
		log.Println(msg)
		result.Message = msg
	} else {
		file.Close()
		msg := fmt.Sprintf("Log file %s is accessible", name)
		log.Println(msg)
		result.Readable = true
		result.Message = msg
	}

	return result
}

func getCollectListFromConfig(config map[string]interface{}) ([]interface{}, error) {
	logs, ok := config["logs"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("logs configuration not found")
	}

	logsCollected, ok := logs["logs_collected"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("logs_collected configuration not found")
	}

	files, ok := logsCollected["files"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("files configuration not found")
	}

	collectList, ok := files["collect_list"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("collect_list configuration not found")
	}

	return collectList, nil
}
