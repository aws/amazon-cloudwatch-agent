// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/agentconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/basicInfo"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/collectd"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration/windows"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/ssm"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/statsd"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/template"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/tracesconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type MainProcessorMock struct {
	mock.Mock
}

func (m *MainProcessorMock) VerifyProcessor(processor interface{}) {
	m.Called(processor)
}

func TestMainMethod(t *testing.T) {
	processors.StartProcessor = template.Processor
	main()
}

func TestWindowsMigration(t *testing.T) {
	// Do the mocking
	processorMock := &MainProcessorMock{}
	processorMock.On("VerifyProcessor", basicInfo.Processor).Return()
	processorMock.On("VerifyProcessor", agentconfig.Processor).Return()
	processorMock.On("VerifyProcessor", statsd.Processor).Return()
	processorMock.On("VerifyProcessor", collectd.Processor).Return()
	processorMock.On("VerifyProcessor", migration.Processor).Return()
	processorMock.On("VerifyProcessor", tracesconfig.Processor).Return()
	processorMock.On("VerifyProcessor", windows.Processor).Return()
	processorMock.On("VerifyProcessor", ssm.Processor).Return()
	MainProcessorGlobal = processorMock

	// Run the functions
	absPath, _ := filepath.Abs("../../tool/processors/migration/windows/testData/input1.json")
	addWindowsMigrationInputs(absPath, "", "", false)
	processors.StartProcessor = basicInfo.Processor

	*isNonInteractiveWindowsMigration = true
	startProcessing()

	// Assert expected behaviour
	assert.True(t, processorMock.AssertNumberOfCalls(t, "VerifyProcessor", 7))

	// Assert the resultant output file as well
	absPath, _ = filepath.Abs("../../tool/processors/migration/windows/testData/output1.json")
	expectedConfig, err := windows.ReadNewConfigFromPath(absPath)
	if err != nil {
		t.Error(err)
		return
	}
	actualConfig, err := windows.ReadNewConfigFromPath(util.ConfigFilePath())
	if err != nil {
		t.Error(err)
		return
	}
	if !windows.AreTwoConfigurationsEqual(actualConfig, expectedConfig) {
		t.Errorf("The generated new config is incorrect, got:\n '%v'\n, want:\n '%v'.\n", actualConfig, expectedConfig)
	}
}
