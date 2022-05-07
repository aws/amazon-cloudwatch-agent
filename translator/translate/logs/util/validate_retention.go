package util

import (
	"fmt"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	logRetentionKey = "retention_in_days"
	logGroupKey     = "log_group_name"
)

func ValidateLogRetentionSettings(logConfigs []interface{}, currPath string) []interface{} {
	configMap := make(map[string]int)
	for _, logConfig := range logConfigs {
		if logConfigMap, ok := logConfig.(map[string]interface{}); ok {
			// skip if retention is not set
			if retention, ok := logConfigMap[logRetentionKey].(int); ok {
				// if retention is 0, -1 or less, it's either invalid or default, skip it
				if retention < 1 {
					continue
				}
				if logGroup, ok := logConfigMap[logGroupKey].(string); ok {
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
						logConfigMap[logRetentionKey] = -1
					} else {
						configMap[logGroup] = retention
					}
				}
			}
		}
	}
	return logConfigs
}
