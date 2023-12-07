// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
	"fmt"
	"strconv"

	"golang.org/x/exp/slices"

	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

// ErrorMessages will provide detail error messages to user
var ErrorMessages = []string{}
var InfoMessages = []string{}

// ValidRetentionInDays is based on what's supported by PutRetentionPolicy. See https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-Configuration-File-Details.html#CloudWatch-Agent-Configuration-File-Logssection.
var ValidRetentionInDays = []string{"-1", "1", "3", "5", "7", "14", "30", "60", "90", "120", "150", "180", "365", "400", "545", "731", "1096", "1827", "2192", "2557", "2922", "3288", "3653"}

var ValidLogGroupClasses = []string{util.StandardLogGroupClass, util.InfrequentAccessLogGroupClass}

// IsValid checks whether the mandatory config parameter is valid
func IsValid(input interface{}, key string, path string) bool {
	m := input.(map[string]interface{})
	val, ok := m[key]
	//Check if the key exists in the input
	if !ok {
		err := key + " field is missed."
		//errMessage := "The path of the error is : " + path + "|" + "Errors :" + err
		errMessage := fmt.Sprintf("The path of the error is : %s | Errors : %s", path, err)
		ErrorMessages = append(ErrorMessages, errMessage)
		return false
	}
	//Check if the value for the key is nil
	if val == nil {
		err := key + " field's value is missed."
		//errMessage := "The path of the error is : " + path + "|" + "Errors :" + err
		errMessage := fmt.Sprintf("The path of the error is : %s | Errors : %s", path, err)
		ErrorMessages = append(ErrorMessages, errMessage)
		return false
	}
	return true
}

func AddErrorMessages(path, message string) {
	var errorMessage string
	if path == "" {
		errorMessage = message
	} else {
		errorMessage = fmt.Sprintf("Under path : %s | Error : %s", path, message)
	}
	ErrorMessages = append(ErrorMessages, errorMessage)
}

func AddInfoMessages(path, message string) {
	var infoMessage string
	if path == "" {
		infoMessage = message
	} else {
		infoMessage = fmt.Sprintf("Under path : %s | Info : %s", path, message)
	}
	InfoMessages = append(InfoMessages, infoMessage)
}

func IsTranslateSuccess() bool {
	return len(ErrorMessages) == 0
}

// Used for testing purpose
func ResetMessages() {
	ErrorMessages = make([]string, 0)
	InfoMessages = make([]string, 0)
}

// ValidDays represents the valid possible values for retentionInDays.
// -1 represents no change in retention

var ValidDays = map[int]bool{}

func initializeValidDaysMap() {
	for i := 0; i < len(ValidRetentionInDays); i++ {
		val, err := strconv.Atoi(ValidRetentionInDays[i])
		if err == nil {
			ValidDays[val] = true
		}
	}
}

func IsValidRetentionDays(days int) bool {
	initializeValidDaysMap()
	return ValidDays[days]
}

func IsValidLogGroupClass(class string) bool {
	return slices.Contains(ValidLogGroupClasses, class) || class == ""
}
