// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapOldWindowsConfigToNewConfig(t *testing.T) {
	testCases := []struct {
		inputFile  string
		outputFile string
	}{
		{inputFile: "testData/input1.json", outputFile: "testData/output1.json"},
		{inputFile: "testData/input2.json", outputFile: "testData/output2.json"},
		{inputFile: "testData/input3.json", outputFile: "testData/output3.json"},
		{inputFile: "testData/input4.json", outputFile: "testData/output4.json"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s, %s", tc.inputFile, tc.outputFile), func(t *testing.T) {
			// Get input
			absPath, _ := filepath.Abs(tc.inputFile)
			oldConfig, err := ReadOldConfigFromPath(absPath)
			if err != nil {
				t.Error(err)
				return
			}

			// Get actual output
			newConfig := MapOldWindowsConfigToNewConfig(oldConfig)
			confBytes, err := json.Marshal(newConfig)
			if err != nil {
				t.Error(err)
				return
			}
			actualJson := string(confBytes)
			// Get the expected
			absPath, _ = filepath.Abs(tc.outputFile)
			expectedJson, err := ReadConfigFromPathAsString(absPath)
			if err != nil {
				t.Error(err)
				return
			}

			// strict compare JSON strings
			assert.JSONEq(t, expectedJson, actualJson)
		})
	}
}

func TestInvalidOldWindowsConfig(t *testing.T) {
	testCases := []struct {
		inputFile string
	}{
		{inputFile: "invalidTestData/input1.json"},
		{inputFile: "invalidTestData/input2.json"},
	}

	for _, tc := range testCases {
		t.Run(tc.inputFile, func(t *testing.T) {
			// Get input
			absPath, _ := filepath.Abs(tc.inputFile)
			oldConfig, err := ReadOldConfigFromPath(absPath)
			if err != nil {
				t.Error(err)
				return
			}

			// Run the function - expect it to exit(1) as the input is invalid
			// Ref: https://stackoverflow.com/questions/26225513/how-to-test-os-exit-scenarios-in-go
			if os.Getenv("BE_CRASHER") == "1" {
				MapOldWindowsConfigToNewConfig(oldConfig)
				return
			}
			cmd := exec.Command(os.Args[0], "-test.run=TestInvalidOldWindowsConfig")
			cmd.Env = append(os.Environ(), "BE_CRASHER=1")
			err = cmd.Run()
			if e, ok := err.(*exec.ExitError); ok && !e.Success() {
				return
			}
			t.Fatalf("process ran with err %v, want exit status 1", err)
		})
	}
}

func TestMapLogLevelsStringToSlice(t *testing.T) {
	levels := mapLogLevelsStringToSlice("7")
	expected := []string{ERROR, WARNING, INFORMATION}

	if !reflect.DeepEqual(levels, expected) {
		t.Errorf("The generated levels are incorrect, got:\n %s\n, want:\n %s.\n", levels, expected)
	}
}
