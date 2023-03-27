// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"runtime"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"

	"os"

	"github.com/stretchr/testify/assert"
)

var expectResult = `{
	"agent": {
		"collect_interval": "10s"
	},
	"metrics": {
		"cpu": {
			"percore": true
		}
	}
}`

func TestCurOS(t *testing.T) {
	assert.Equal(t, runtime.GOOS, CurOS())
}

func TestReadConfigFromJsonFile(t *testing.T) {
	err := os.WriteFile(ConfigFilePath(), []byte(expectResult), os.ModePerm)
	assert.NoError(t, err)

	actualResult := ReadConfigFromJsonFile()
	assert.Equal(t, expectResult, actualResult)
}

func TestSerializeResultMapToJsonByteArray(t *testing.T) {
	resultMap := make(map[string]interface{})

	agentMap := make(map[string]interface{})
	resultMap["agent"] = agentMap
	agentMap["collect_interval"] = "10s"

	metricsMap := make(map[string]interface{})
	resultMap["metrics"] = metricsMap
	cpuMap := make(map[string]interface{})
	metricsMap["cpu"] = cpuMap
	cpuMap["percore"] = true

	bytes := SerializeResultMapToJsonByteArray(resultMap)
	assert.Equal(t, expectResult, string(bytes))

}

func TestSaveResultByteArrayToJsonFile(t *testing.T) {
	filePath := SaveResultByteArrayToJsonFile([]byte(expectResult))
	bytes, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	actualResult := string(bytes)
	assert.Equal(t, expectResult, actualResult)
}

func TestYes(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")
	assert.True(t, Yes("Some question"))

	testutil.Type(inputChan, "2")
	assert.False(t, Yes("Some question"))
}

func TestNo(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")
	assert.False(t, No("Some question"))

	testutil.Type(inputChan, "1")
	assert.True(t, No("Some question"))
}

func TestAskWithDefault(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")

	parsedAnswer := AskWithDefault("Question", "DefaultAnswer")

	assert.Equal(t, "DefaultAnswer", parsedAnswer)

	testutil.Type(inputChan, "Answer")

	parsedAnswer = AskWithDefault("Question", "DefaultAnswer")

	assert.Equal(t, "Answer", parsedAnswer)
}

func TestAsk(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "Answer")

	parsedAnswer := Ask("Question")

	assert.Equal(t, "Answer", parsedAnswer)
}

func TestChoice(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")

	parsedAnswer := Choice("Question", 1, []string{"validValue1", "validValue2"})

	assert.Equal(t, "validValue1", parsedAnswer)

	testutil.Type(inputChan, "InvalidAnswer", "2")

	parsedAnswer = Choice("Question", 1, []string{"validValue1", "validValue2"})

	assert.Equal(t, "validValue2", parsedAnswer)
}
