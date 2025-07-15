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
	printMessages(DebuggerErrorCollector.ConfigErrors, "Errors", "❌")
	printMessages(DebuggerErrorCollector.ConfigWarnings, "Warnings", "⚠️")
}

func printMessages(messages []string, title, icon string) {
	if len(messages) == 0 {
		return
	}
	fmt.Printf("%s (%d):\n", title, len(messages))
	for _, msg := range messages {
		fmt.Printf("  %s %s\n", icon, msg)
	}
}
