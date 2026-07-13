// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package wizard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/basicInfo"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration/windows"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type MainProcessorMock struct {
	mock.Mock
}

func (m *MainProcessorMock) VerifyProcessor(processor interface{}) {
	m.Called(processor)
}

func TestWindowsMigration(t *testing.T) {
	// Do the mocking
	processorMock := &MainProcessorMock{}
	processorMock.On("VerifyProcessor", mock.Anything).Return()
	MainProcessorGlobal = processorMock

	// Set up the test input file path
	absPath, err := filepath.Abs("../../tool/processors/migration/windows/testData/input1.json")
	assert.NoError(t, err, "Failed to get absolute path for input file")

	// Verify that the input file exists
	_, err = os.Stat(absPath)
	assert.NoError(t, err, "Input file does not exist: %s", absPath)

	// Run the wizard
	params := Params{
		IsNonInteractiveWindowsMigration: true,
		ConfigFilePath:                   absPath,
	}
	processors.StartProcessor = basicInfo.Processor
	err = RunWizard(params)

	// Assert no error occurred
	assert.NoError(t, err, "RunWizard returned an error")

	// Assert expected behaviour
	callCount := processorMock.Calls
	assert.Equal(t, 7, len(callCount), "Expected 7 calls to VerifyProcessor, got %d", len(callCount))

	// Assert the resultant output file
	outputPath, err := filepath.Abs("../../tool/processors/migration/windows/testData/output1.json")
	assert.NoError(t, err, "Failed to get absolute path for output file")

	// Verify that the output file exists
	_, err = os.Stat(outputPath)
	assert.NoError(t, err, "Output file does not exist: %s", outputPath)

	expectedConfig, err := windows.ReadNewConfigFromPath(outputPath)
	if err != nil {
		t.Fatalf("Failed to read expected config: %v", err)
	}

	actualConfigPath := util.ConfigFilePath()
	t.Logf("Actual config path: %s", actualConfigPath)

	// Verify that the actual config file exists and is not empty
	actualConfigInfo, err := os.Stat(actualConfigPath)
	assert.NoError(t, err, "Actual config file does not exist: %s", actualConfigPath)
	assert.NotEqual(t, 0, actualConfigInfo.Size(), "Actual config file is empty")

	actualConfig, err := windows.ReadNewConfigFromPath(actualConfigPath)
	if err != nil {
		t.Fatalf("Failed to read actual config: %v", err)
	}

	assert.True(t, windows.AreTwoConfigurationsEqual(actualConfig, expectedConfig),
		"The generated new config is incorrect, got: '%v', want: '%v'.", actualConfig, expectedConfig)
}
