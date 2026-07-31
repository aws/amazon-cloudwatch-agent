// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/timestamp"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

type TimestampRegex struct {
}

// ApplyRule add timestamp regex
// do not add timestamp check when viewing cwa logfile
func (t *TimestampRegex) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	//Convert the input string into []rune and iterate the map and build the output []rune
	m := input.(map[string]interface{})
	//If user not specify the timestamp_format, then no config entry for "timestamp_layout" in TOML
	if val, ok := m["timestamp_format"]; !ok {
		return "", ""
	} else if m["file_path"] == context.CurrentContext().GetAgentLogFile() {
		fmt.Printf("timestamp_format set file_path : %s is the same as agent log file %s thus do not use timestamp_regex \n", m["file_path"], context.CurrentContext().GetAgentLogFile())
		return "", ""
	} else {
		//If user provide with the specific timestamp_format, use the one that user provide
		res := "(" + timestamp.BuildRegex(val.(string)) + ")"
		returnKey = "timestamp_regex"
		if _, err := regexp.Compile(res); err != nil {
			translator.AddErrorMessages(GetCurPath()+"timestamp_format", fmt.Sprintf("Timestamp format %s is invalid", val))
			return
		}
		returnVal = res
	}
	return
}

type TimestampLayout struct {
}

// ApplyRule add timestamp layout
// do not add timestamp check when viewing cwa logfile
func (t *TimestampLayout) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	//Convert the input string into []rune and iterate the map and build the output []rune
	m := input.(map[string]interface{})
	//If user not specify the timestamp_format, then no config entry for "timestamp_layout" in TOML
	if val, ok := m["timestamp_format"]; !ok {
		return "", ""
	} else if m["file_path"] == context.CurrentContext().GetAgentLogFile() {
		fmt.Printf("timestamp_format set file_path : %s is the same as agent log file %s thus do not use timestamp_layout \n", m["file_path"], context.CurrentContext().GetAgentLogFile())
		return "", ""
	} else {
		res := timestamp.ReplaceAll(val.(string), timestamp.FormatLayoutMap)
		//If user provide with the specific timestamp_format, use the one that user provide
		returnKey = "timestamp_layout"
		timestampInput := val.(string)
		// Go doesn't support _2 option for month in day as a result need to set
		// timestamp_layout with 2 strings which support %m and %-m
		if strings.Contains(timestampInput, "%m") {
			timestampInput = strings.ReplaceAll(timestampInput, "%m", "%-m")
			alternativeLayout := timestamp.ReplaceAll(timestampInput, timestamp.FormatLayoutMap)
			returnVal = []string{res, alternativeLayout}
		} else if strings.Contains(timestampInput, "%-m") {
			timestampInput = strings.ReplaceAll(timestampInput, "%-m", "%m")
			alternativeLayout := timestamp.ReplaceAll(timestampInput, timestamp.FormatLayoutMap)
			returnVal = []string{res, alternativeLayout}
		} else {
			returnVal = []string{res}
		}
	}
	return
}

type Timezone struct {
}

func (t *Timezone) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if val, ok := m["timezone"]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If user provide with the specific timestamp_format, use the one that user provide
		returnKey = "timezone"
		if val == "UTC" {
			returnVal = "UTC"
		} else {
			returnVal = "LOCAL"
		}
	}
	return
}
func init() {
	t1 := new(TimestampLayout)
	t2 := new(TimestampRegex)
	t3 := new(Timezone)
	r := []Rule{t1, t2, t3}
	RegisterRule("timestamp_format", r)
}
