// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util //nolint:revive // existing package name

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestBaseName(t *testing.T) {
	testCases := map[string]struct {
		path string
		want string
	}{
		"WithEmptyPath": {
			path: "",
			want: "",
		},
		"WithSimplePath": {
			path: filepath.Join("usr", "bin", "test.jar"),
			want: "test.jar",
		},
		"WithQuotedPath": {
			path: fmt.Sprintf("%q", filepath.Join("usr", "bin", "TEST.jar")),
			want: "TEST.jar",
		},
		"WithDeleted": {
			path: filepath.Join("usr", "bin", "test.jar (deleted)"),
			want: "test.jar",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := BaseName(testCase.path)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestBaseExe(t *testing.T) {
	testCases := map[string]struct {
		exe  string
		want string
	}{
		"WithEmptyPath": {
			exe:  "",
			want: "",
		},
		"WithSimplePath": {
			exe:  filepath.Join("usr", "bin", "java"),
			want: "java",
		},
		"WithQuotedPath": {
			exe:  fmt.Sprintf("%q", filepath.Join("usr", "bin", "Java")),
			want: "java",
		},
		"WithExtension": {
			exe:  filepath.Join("usr", "bin", "java.exe"),
			want: "java",
		},
		"WithDeleted": {
			exe:  filepath.Join("usr", "bin", "java.exe (deleted)"),
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

func TestAbsPath(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	testCases := map[string]struct {
		path    string
		setup   func(*detectortest.MockProcess)
		want    string
		wantErr error
	}{
		"AbsolutePath": {
			path:  filepath.Join(tmpDir, "file.txt"),
			setup: func(*detectortest.MockProcess) {},
			want:  filepath.Join(tmpDir, "file.txt"),
		},
		"RelativePath": {
			path: "relative/file.txt",
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CwdWithContext", ctx).Return(tmpDir, nil)
			},
			want: filepath.Join(tmpDir, "relative/file.txt"),
		},
		"RelativePathWithDot": {
			path: "./file.txt",
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CwdWithContext", ctx).Return(tmpDir, nil)
			},
			want: filepath.Join(tmpDir, "file.txt"),
		},
		"Process/Error": {
			path: "relative/file.txt",
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CwdWithContext", ctx).Return("", assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := &detectortest.MockProcess{}
			testCase.setup(mp)

			got, err := AbsPath(ctx, mp, testCase.path)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
			mp.AssertExpectations(t)
		})
	}
}
