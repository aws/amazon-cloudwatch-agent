// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package globpath

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInversionGlobPattern(t *testing.T) {
	dir := getTestdataDir()
	g, err := Compile(dir + "/log[!23].log")
	require.NoError(t, err)
	assert.NotNil(t, g)

	matches := g.Match()
	assert.Len(t, matches, 1)
	stat, ok := matches[dir+"/log1.log"]
	assert.True(t, ok, "The matched file should NOT be log2.log")
	assert.NotNil(t, stat)
	assert.Equal(t, "log1.log", stat.Name())
}

// TestCompileGlob ensures that different path configurations will
// set the glob.Glob attribute of the GlobPath struct, and ensures that
// the glob is used to match files appropriately
func TestCompileGlob(t *testing.T) {
	dir := getTestdataDir()

	tests := []struct {
		path         string
		shouldError  bool
		hasGlob      bool
		hasMeta      bool
		hasSuperMeta bool
		numMatched   int
	}{
		{
			path:       "/nested1/nested2/nested.txt",
			numMatched: 1,
		},
		{
			path:       "/log[!23].log",
			hasGlob:    true,
			hasMeta:    true,
			numMatched: 1,
		},
		{
			path:       "/log?.log",
			hasGlob:    true,
			hasMeta:    true,
			numMatched: 2,
		},
		{
			path:         "/**",
			hasGlob:      true,
			hasMeta:      true,
			hasSuperMeta: true,
			numMatched:   6,
		},
		{
			path:        "/[something?",
			shouldError: true,
		},
		{
			path:         "/**/{[n-z]ested, foo}.txt",
			hasGlob:      true,
			hasMeta:      true,
			hasSuperMeta: true,
			numMatched:   1,
		},
		{
			path:       "/i_dont_exist.log",
			numMatched: 0,
		},
		{
			path:         "/dir_doesnt_exist/**",
			hasGlob:      true,
			hasMeta:      true,
			hasSuperMeta: true,
			numMatched:   0,
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			g, err := Compile(dir + test.path)
			if test.shouldError {
				assert.Error(t, err)
				assert.Nil(t, g)
				return // short-circuit
			}

			assert.NotNil(t, g)
			assert.NoError(t, err)
			if test.hasGlob {
				assert.NotNil(t, g.g)
			} else {
				assert.Nil(t, g.g)
			}

			assert.Equal(t, test.hasMeta, g.hasMeta)
			assert.Equal(t, test.hasSuperMeta, g.hasSuperMeta)
			assert.Equal(t, test.numMatched, len(g.Match()))
		})
	}
}

func TestCompileAndMatch(t *testing.T) {
	dir := getTestdataDir()
	// test super asterisk
	g1, err := Compile(dir + "/**")
	require.NoError(t, err)
	// test single asterisk
	g2, err := Compile(dir + "/*.log")
	require.NoError(t, err)
	// test no meta characters (file exists)
	g3, err := Compile(dir + "/log1.log")
	require.NoError(t, err)
	// test file that doesn't exist
	g4, err := Compile(dir + "/i_dont_exist.log")
	require.NoError(t, err)
	// test super asterisk that doesn't exist
	g5, err := Compile(dir + "/dir_doesnt_exist/**")
	require.NoError(t, err)

	matches := g1.Match()
	assert.Len(t, matches, 6)
	matches = g2.Match()
	assert.Len(t, matches, 2)
	matches = g3.Match()
	assert.Len(t, matches, 1)
	matches = g4.Match()
	assert.Len(t, matches, 0)
	matches = g5.Match()
	assert.Len(t, matches, 0)
}

func TestFindRootDir(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"/var/log/telegraf.conf", "/var/log"},
		{"/home/**", "/home"},
		{"/home/*/**", "/home"},
		{"/lib/share/*/*/**.txt", "/lib/share"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			actual := findRootDir(test.input)
			assert.Equal(t, test.output, actual)
		})
	}
}

func TestFindNestedTextFile(t *testing.T) {
	dir := getTestdataDir()
	// test super asterisk
	g1, err := Compile(dir + "/**.txt")
	require.NoError(t, err)

	matches := g1.Match()
	assert.Len(t, matches, 1)
}

func getTestdataDir() string {
	_, filename, _, _ := runtime.Caller(1)
	return strings.Replace(filename, "globpath_test.go", "testdata", 1)
}
