// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/files"
)

type Rule translator.Rule

const (
	SectionKey              = "collect_list"
	logConfigOutputFileName = "log-config.json"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

var Index = 0
var ChildRule = map[string][]Rule{}

func RegisterRule(fieldname string, r []Rule) {
	ChildRule[fieldname] = r
}

//resetIndex resets the state of the Index.
func resetIndex() {
	Index = 0
}

type FileConfig struct {
}

func (f *FileConfig) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	res := []interface{}{}
	//if translator.IsValid(input, SectionKey, CurrentPath+SectionKey) {
	if translator.IsValid(input, SectionKey, GetCurPath()) {
		configArr := m[SectionKey].([]interface{})
		for i := 0; i < len(configArr); i++ {
			Index += 1
			result := map[string]interface{}{}
			for _, ruleArr := range ChildRule {
				for j := 0; j < len(ruleArr); j++ {
					key, val := ruleArr[j].ApplyRule(configArr[i])
					if key != "" {
						result[key] = val
					}
				}
			}
			res = append(res, result)
		}
		checkForConflictingRetentionSettings(res)
		outputLogConfig(res)
	} else {
		returnKey = ""
		returnVal = ""
	}
	returnKey = "file_config"
	returnVal = res
	resetIndex()
	return
}

type OutputLogConfigFile struct {
	Version    string      `json:"version"`
	LogConfigs []LogConfig `json:"log_configs"`
	Region     string      `json:"region"`
}

type LogConfig struct {
	LogGroupName string `json:"log_group_name"`
}

func checkForConflictingRetentionSettings(logConfigs []interface{}) []interface{} {
	configMap := make(map[string]int)
	for _, logConfig := range logConfigs {
		if logConfigMap, ok := logConfig.(map[string]interface{}); ok {
			logGroup := strings.ToLower(logConfigMap[LogGroupNameSectionKey].(string))
			// if retention is 0, -1 or less it's either invalid or default
			retention := logConfigMap["retention_in_days"].(int)
			if retention < 1 {
				continue
			}
			// if the configMap[logGroup] exists, retention has been set for the same logGroup somewhere
			if configMap[logGroup] != 0 {
				// different retentions has been set for the same log group, panic and stop the agent
				if configMap[logGroup] != retention {
					translator.AddErrorMessages(
						GetCurPath()+SectionKey,
						fmt.Sprintf("Different Retention values can't be set for the same log group: %v", logGroup))
				}
				// The same retention for a log group has been configured in multiple places. Unset it so that the retention api is only called once
				logConfigMap["retention_in_days"] = -1
			} else {
				configMap[logGroup] = retention
			}
		}
	}
	return logConfigs
}

func outputLogConfig(logConfigs []interface{}) {
	if context.CurrentContext().OutputTomlFilePath() == "" {
		return
	}
	//output log config info (currently only log group names) to the same folder as toml output file
	outputLogConfigFilePath := filepath.Join(
		filepath.Dir(context.CurrentContext().OutputTomlFilePath()),
		logConfigOutputFileName)
	//use map to remove duplicate LogConfig
	outputMap := map[LogConfig]interface{}{}
	for _, logConfig := range logConfigs {
		if logConfigMap, ok := logConfig.(map[string]interface{}); ok {
			if name, ok := logConfigMap[LogGroupNameSectionKey]; ok {
				if nameStr, ok := name.(string); ok {
					outputMap[LogConfig{LogGroupName: nameStr}] = nil
				}
			}
		}
	}
	//use list to stabilize the output
	outputList := make([]LogConfig, 0, len(outputMap))
	for logConfig := range outputMap {
		outputList = append(outputList, logConfig)
	}
	sort.SliceStable(outputList, func(i, j int) bool {
		return outputList[i].LogGroupName < outputList[j].LogGroupName
	})
	outputFile := &OutputLogConfigFile{
		Version:    "1",
		LogConfigs: outputList,
		Region:     agent.Global_Config.Region,
	}
	if bytes, err := json.Marshal(outputFile); err == nil {
		ioutil.WriteFile(outputLogConfigFilePath, bytes, 0644)
	}
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (f *FileConfig) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeList(source, result, SectionKey)
}

func init() {
	f := new(FileConfig)
	parent.RegisterRule(SectionKey, f)
	parent.MergeRuleMap[SectionKey] = f
}
