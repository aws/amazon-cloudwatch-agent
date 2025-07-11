// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import "fmt"

type ErrorCollector struct {
	ConfigErrors   []string
	ConfigWarnings []string
}

var DebuggerErrorCollector = &ErrorCollector{}

func AddConfigError(msg string) {
	DebuggerErrorCollector.ConfigErrors = append(DebuggerErrorCollector.ConfigErrors, msg)
}

func AddConfigWarning(msg string) {
	DebuggerErrorCollector.ConfigWarnings = append(DebuggerErrorCollector.ConfigWarnings, msg)
}

func PrintAggregatedErrors() {
	fmt.Println("=== Errors & Warnings Summary ===")
	if len(DebuggerErrorCollector.ConfigErrors) > 0 || len(DebuggerErrorCollector.ConfigWarnings) > 0 {

		if len(DebuggerErrorCollector.ConfigErrors) > 0 {
			fmt.Printf("Errors (%d):\n", len(DebuggerErrorCollector.ConfigErrors))
			for _, err := range DebuggerErrorCollector.ConfigErrors {
				fmt.Printf("  ❌ %s\n", err)
			}
		}

		if len(DebuggerErrorCollector.ConfigWarnings) > 0 {
			fmt.Printf("Warnings (%d):\n", len(DebuggerErrorCollector.ConfigWarnings))
			for _, warn := range DebuggerErrorCollector.ConfigWarnings {
				fmt.Printf("  ⚠️  %s\n", warn)
			}
		}
	}
}
