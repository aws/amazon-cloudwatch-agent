// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/processors/defaultConfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

func TestNextProcessor(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	testutil.Type(inputChan, "2")
	assert.Equal(t, defaultConfig.Processor, Processor.NextProcessor(nil, nil))
}

func TestMigrateOldAgentConfigPanic(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	testutil.Type(inputChan, "wrongFilePath")

	// Ref: https://stackoverflow.com/questions/26225513/how-to-test-os-exit-scenarios-in-go
	if os.Getenv("BE_CRASHER") == "1" {
		migrateOldAgentConfig()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMigrateOldAgentConfigPanic")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestMigrateOldAgentConfigCorrectFile(t *testing.T) {
	absPath, _ := filepath.Abs("testData/input1.json")
	inputChan := testutil.SetUpTestInputStream()
	testutil.Type(inputChan, absPath)
	migrateOldAgentConfig()
}
