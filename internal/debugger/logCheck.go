// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"fmt"
	"os"
	"path/filepath"
)

func CheckLogs(config map[string]interface{}) {
	fmt.Println("Running CheckLogs...")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Panicked with error")
			return
		}
	}()

	//go to logs.logs_collected.files.collect_list in json
	logs := config["logs"].(map[string]interface{})
	logsCollected := logs["logs_collected"].(map[string]interface{})
	files := logsCollected["files"].(map[string]interface{})
	collectList := files["collect_list"].([]interface{})

	if len(collectList) == 0 {
		fmt.Println("Nothing in collectList")
	}

	for _, item := range collectList {
		itemMap := item.(map[string]interface{})
		filePath := itemMap["file_path"].(string)
		checkLogPermissions(filePath)
	}
}

// Checking for existence and readability
func checkLogPermissions(filePath string) {
	name := filepath.Base(filePath)
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("Configured log file %s does not exist.\n", name)
		return
	}

	if file, err := os.Open(filePath); err != nil {
		fmt.Printf("Agent does not have read permission for log file %s\n", name)
		fmt.Printf("Try: sudo chmod 644 %s\n", filePath)
	} else {
		file.Close()
		fmt.Printf("Log file %s is accessible\n", name)
	}
}
