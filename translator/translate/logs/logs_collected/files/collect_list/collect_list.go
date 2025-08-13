// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/files"
	logUtil "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
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

// resetIndex resets the state of the Index.
func resetIndex() {
	Index = 0
}

type FileConfig struct {
}

func (f *FileConfig) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	res := []interface{}{}
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
		logUtil.ValidateLogGroupFields(res, GetCurPath())
		outputLogConfig(res)
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
		os.WriteFile(outputLogConfigFilePath, bytes, 0644)
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
