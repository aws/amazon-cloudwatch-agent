// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import "fmt"

type ErrorCollector struct {
	ConfigErrors   []string
	ConfigWarnings []string
}

var errorCollector *ErrorCollector

func GetErrorCollector() *ErrorCollector {
	if errorCollector == nil {
		errorCollector = &ErrorCollector{}
	}
	return errorCollector
}

func (ec *ErrorCollector) AddError(msg string) {
	ec.ConfigErrors = append(ec.ConfigErrors, msg)
}

func (ec *ErrorCollector) AddWarning(msg string) {
	ec.ConfigWarnings = append(ec.ConfigWarnings, msg)
}

func (ec *ErrorCollector) PrintErrors() {
	fmt.Println("=== Config Errors & Warnings Summary ===")
	ec.printErrorMessages(ec.ConfigErrors, "Errors")
	ec.printErrorMessages(ec.ConfigWarnings, "Warnings")
}

func (ec *ErrorCollector) printErrorMessages(messages []string, title string) {

	if len(messages) == 0 {
		fmt.Printf("No %s\n", title)
		return
	}

	fmt.Printf("%s (%d):\n", title, len(messages))
	for _, msg := range messages {
		fmt.Printf("  %s\n", msg)
	}

}
