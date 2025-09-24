// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseExe(t *testing.T) {
	testCases := map[string]struct {
		exe  string
		want string
	}{
		"EmptyString": {
			exe:  "",
			want: "",
		},
		"SimpleExe": {
			exe:  filepath.Join("usr", "bin", "java"),
			want: "java",
		},
		"QuotedPath": {
			exe:  fmt.Sprintf("%q", filepath.Join("usr", "bin", "java")),
			want: "java",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := BaseExe(testCase.exe)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestTrimQuotes(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  string
	}{
		"EmptyString": {
			input: "",
			want:  "",
		},
		"DoubleQuotes": {
			input: `"hello"`,
			want:  "hello",
		},
		"SingleQuotes": {
			input: `'world'`,
			want:  "world",
		},
		"NoQuotes": {
			input: "text",
			want:  "text",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := TrimQuotes(testCase.input)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestIsValidPort(t *testing.T) {
	testCases := map[string]struct {
		port int
		want bool
	}{
		"ValidPort": {
			port: 8080,
			want: true,
		},
		"InvalidPort/Negative": {
			port: -1,
			want: false,
		},
		"InvalidPort/TooLarge": {
			port: 65536,
			want: false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := IsValidPort(testCase.port)
			assert.Equal(t, testCase.want, got)
		})
	}
}
