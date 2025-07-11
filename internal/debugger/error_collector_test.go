// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintAggregatedErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []string
		warnings []string
		contains []string
	}{
		{
			name:     "No errors or warnings",
			errors:   []string{},
			warnings: []string{},
			contains: []string{"=== Errors & Warnings Summary ==="},
		},
		{
			name:     "Only errors",
			errors:   []string{"error 1", "error 2"},
			warnings: []string{},
			contains: []string{"Errors (2):", "❌ error 1", "❌ error 2"},
		},
		{
			name:     "Only warnings",
			errors:   []string{},
			warnings: []string{"warning 1"},
			contains: []string{"Warnings (1):", "⚠️  warning 1"},
		},
		{
			name:     "Both errors and warnings",
			errors:   []string{"error 1"},
			warnings: []string{"warning 1"},
			contains: []string{"Errors (1):", "❌ error 1", "Warnings (1):", "⚠️  warning 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset and populate collector
			DebuggerErrorCollector = &ErrorCollector{}
			DebuggerErrorCollector.ConfigErrors = tt.errors
			DebuggerErrorCollector.ConfigWarnings = tt.warnings

			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			PrintAggregatedErrors()

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			for _, expected := range tt.contains {
				assert.Contains(t, output, expected)
			}
		})
	}
}

func TestErrorCollectorIntegration(t *testing.T) {
	// Reset collector
	DebuggerErrorCollector = &ErrorCollector{}

	AddConfigError("critical error")
	AddConfigWarning("minor warning")
	AddConfigError("another error")

	assert.Len(t, DebuggerErrorCollector.ConfigErrors, 2)
	assert.Len(t, DebuggerErrorCollector.ConfigWarnings, 1)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintAggregatedErrors()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "Errors (2):")
	assert.Contains(t, output, "Warnings (1):")
	assert.Contains(t, output, "critical error")
	assert.Contains(t, output, "minor warning")
	assert.Contains(t, output, "another error")
}


