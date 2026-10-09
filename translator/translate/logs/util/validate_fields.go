// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	logRetentionKey  = "retention_in_days"
	logGroupKey      = "log_group_name"
	logGroupClassKey = "log_group_class"
	kmsKeyIdKey      = "kms_key_id"
)

func ValidateLogGroupFields(logConfigs []interface{}, currPath string) []interface{} {
	logConfigs = validateLogRetentionSettings(logConfigs, currPath)
	logConfigs = validateLogGroupClassSettings(logConfigs, currPath)
	logConfigs = validateKmsKeySettings(logConfigs, currPath)
	return logConfigs
}

func validateKmsKeySettings(logConfigs []interface{}, currPath string) []interface{} {
	seen := make(map[string]string)
	present := make(map[string]bool)
	for _, logConfig := range logConfigs {
		logConfigMap, ok := logConfig.(map[string]interface{})
		if !ok {
			continue
		}
		logGroup, ok := logConfigMap[logGroupKey].(string)
		if !ok || logGroup == "" {
			continue
		}
		logGroup = strings.ToLower(logGroup)
		kmsKey, _ := logConfigMap[kmsKeyIdKey].(string)
		if present[logGroup] && seen[logGroup] != kmsKey {
			translator.AddErrorMessages(
				currPath,
				fmt.Sprintf("Different kms_key_id values can't be set for the same log group: %v", logGroup))
			continue
		}
		seen[logGroup] = kmsKey
		present[logGroup] = true
	}
	return logConfigs
}

func validateLogRetentionSettings(logConfigs []interface{}, currPath string) []interface{} {
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
					} else {
						configMap[logGroup] = retention
					}
				}
			}
		}
	}
	return logConfigs
}

func validateLogGroupClassSettings(logConfigs []interface{}, currPath string) []interface{} {
	configMap := make(map[string]string)
	for _, logConfig := range logConfigs {
		if logConfigMap, ok := logConfig.(map[string]interface{}); ok {
			// skip if logGroupClass is not set
			if logGroupClass, ok := logConfigMap[logGroupClassKey].(string); ok {
				// if logGroupClass is not one of the proper values, it's invalid skip it
				if !slices.Contains(translator.ValidLogGroupClasses, logGroupClass) {
					continue
				}
				if logGroup, ok := logConfigMap[logGroupKey].(string); ok {
					logGroup = strings.ToLower(logGroup)
					// if the configMap[logGroup] exists, logGroupClass config for the log group was already included earlier
					if configMap[logGroup] != "" {
						// different Log Group Classes are attempted to be configured for the same log group, add error message to fail translation
						if configMap[logGroup] != logGroupClass {
							translator.AddErrorMessages(
								currPath,
								fmt.Sprintf("Different log_group_class values can't be set for the same log group: %v", logGroup))
						}
					} else {
						configMap[logGroup] = logGroupClass
					}
				}
			}
		}
	}
	return logConfigs
}
