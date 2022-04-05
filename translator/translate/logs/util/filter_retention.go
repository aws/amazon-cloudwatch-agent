package util

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"strings"
)

func CheckForConflictingRetentionSettings(logConfigs []interface{}, currPath string) []interface{} {
	configMap := make(map[string]int)
	for _, logConfig := range logConfigs {
		if logConfigMap, ok := logConfig.(map[string]interface{}); ok {
			// skip if retention is not set
			if retention, ok := logConfigMap["retention_in_days"].(int); ok {
				// if retention is 0, -1 or less, it's either invalid or default, skip it
				if retention < 1 {
					continue
				}
				if logGroup, ok := logConfigMap["log_group_name"].(string); ok {
					logGroup = strings.ToLower(logGroup)
					// if the configMap[logGroup] exists, retention config for the log group was already included earlier
					if configMap[logGroup] != 0 {
						// different retentions are attempted to be configured for the same log group, add error message to fail translation
						if configMap[logGroup] != retention {
							translator.AddErrorMessages(
								currPath,
								fmt.Sprintf("Different retention_in_days values can't be set for the same log group: %v", logGroup))
						}
						// Retention for a log group has been configured in multiple places. Unset it so that the retention api is only called once
						logConfigMap["retention_in_days"] = -1
					} else {
						configMap[logGroup] = retention
					}
				}
			}
		}
	}
	return logConfigs
}

