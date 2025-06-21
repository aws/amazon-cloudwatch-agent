package debugger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConfigFile struct {
	Path        string
	Description string
	Required    bool
}

func CheckConfigFiles() {
	fmt.Println("Checking Configuration Files:")

	configFiles := []ConfigFile{
		{
			Path:        "/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.toml",
			Description: "Main TOML configuration file",
			Required:    true,
		},
		{
			Path:        "/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.d",
			Description: "JSON configuration file",
			Required:    true,
		},
		{
			Path:        "/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.yaml",
			Description: "YAML configuration file",
			Required:    true,
		},
		{
			Path:        "/opt/aws/amazon-cloudwatch-agent/etc/common-config.toml",
			Description: "Common configuration file",
			Required:    false,
		},
		{
			Path:        "/opt/aws/amazon-cloudwatch-agent/etc/log-config.json",
			Description: "Log configuration file",
			Required:    false,
		},
		{
			Path:        "/opt/aws/amazon-cloudwatch-agent/etc/env-config.json",
			Description: "Environment configuration file",
			Required:    false,
		},
	}

	for _, file := range configFiles {
		if strings.HasSuffix(file.Path, ".d") {
			// Special handling for .d directory
			entries, err := os.ReadDir(file.Path)
			status := "✓ Present"

			if err != nil {
				if os.IsNotExist(err) {
					status = "✗ Missing"
				} else {
					status = fmt.Sprintf("? Error checking directory: %v", err)
				}
			} else {
				if len(entries) == 0 {
					status = "✗ No file found"
				} else if len(entries) > 1 {
					status = "! Multiple files found"
				} else {
					// Assume the single file is JSON and try to parse it
					fileName := entries[0].Name()
					content, err := os.ReadFile(filepath.Join(file.Path, fileName))
					if err != nil {
						status = "! File present but not readable"
					} else {
						var js map[string]interface{}
						if err := json.Unmarshal(content, &js); err != nil {
							status = "! Existent but invalid JSON"
						}
					}
				}
			}

			fmt.Printf("%-20s [%s] - %s\n",
				"amazon-cloudwatch-agent.d",
				status,
				file.Description)
		} else {
			// Original file checking logic for non-.d paths
			_, err := os.Stat(file.Path)
			status := "✓ Present"
			if err != nil {
				if os.IsNotExist(err) {
					status = "✗ Missing"
				} else {
					status = fmt.Sprintf("? Error checking file: %v", err)
				}
			}

			if status == "✓ Present" {
				content, err := os.ReadFile(file.Path)
				if err != nil {
					status = "! Present but not readable"
				} else {
					if strings.HasSuffix(file.Path, ".json") {
						var js map[string]interface{}
						if err := json.Unmarshal(content, &js); err != nil {
							status = "! Invalid JSON format"
						}
					}
				}
			}

			fmt.Printf("%-20s [%s] - %s\n",
				filepath.Base(file.Path),
				status,
				file.Description)
		}
	}
}
