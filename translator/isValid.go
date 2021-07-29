// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
	"fmt"
)

//ErrorMessages will provide detail error messages to user
var ErrorMessages = []string{}
var InfoMessages = []string{}

//IsValid checks wether the mandatory config parameter is valid
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

// ValidDays represents the valid possible values for retentionInDays
var ValidDays = map[int]bool{1: true, 3: true, 5: true, 7: true, 14: true,
	30: true, 60: true, 90: true, 120: true, 150: true, 180: true,
	365: true, 400: true, 545: true, 731: true, 1827: true, 3653: true}

func IsValidRetentionDays(days int) bool {
	return ValidDays[days]
}
