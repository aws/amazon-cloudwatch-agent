// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jsonconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/totomlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"

	"github.com/stretchr/testify/assert"
)

type TestData struct {
	testName            string
	testId              int
	inputJsonFileNumber int
	shouldFail          bool
}

var testDataList = []TestData{
	{"SeparateSection_CompleteLinuxConfig", 1, 4, false},
	{"SeparateSection_CompleteWindowsConfig", 2, 5, false},
	{"MixedSection_CompleteLinuxConfig", 3, 3, false},
	{"MixedSection_CompleteWindowsConfig", 4, 4, false},
	{"CustomizedTest_PutWhateverYouWantToQuicklyTestHere", 5, 2, false},

	{"FailureTest_AgentConflicts", 6, 2, true},
	{"FailureTest_MetricsConflicts", 7, 2, true},
	{"FailureTest_LogsConflicts", 8, 2, true},
	{"FailureTest_CsmConflicts", 9, 2, true},
	{"MixedSection_LogsMetricCollectedConfig", 10, 2, false},
	{"SeparateSection_LogsMetricAndLog", 11, 2, false},
	{"SeparateSection_PrometheusAndLog", 12, 2, false},
	{"Two_procstat", 13, 2, false},
}

func TestMergeJsonConfigMaps(t *testing.T) {
	for _, testData := range testDataList {
		translator.ResetMessages()
		executeTest(t, testData)
	}
	translator.ResetMessages()
}

func executeTest(t *testing.T, testData TestData) {
	log.Printf("Test %v %v started", testData.testId, testData.testName)

	defer shouldFail(t, testData)

	jsonConfigMapMap := make(map[string]map[string]interface{})
	for i := 0; i < testData.inputJsonFileNumber; i++ {
		jsonFileName := fmt.Sprintf("./sampleJsonConfig/test_%v/input_%v.json", testData.testId, i+1)
		jsonConfigMap, err := util.GetJsonMapFromFile(jsonFileName)
		if err != nil {
			t.Fatalf("Failed to get json map from %v with error: %v", jsonFileName, err)
		}
		jsonConfigMapMap[jsonFileName] = jsonConfigMap
	}

	resultMap, err := MergeJsonConfigMaps(jsonConfigMapMap, nil, "default")
	if err != nil {
		t.Fatalf("Failed to merge json maps with error: %v", err)
	}
	expectedFileName := fmt.Sprintf("./sampleJsonConfig/test_%v/expected_output.json", testData.testId)
	expectedOutputBytes, err := os.ReadFile(expectedFileName)
	if err != nil {
		t.Fatalf("Failed to read expected output file %v with error: %v", expectedFileName, err)
	}

	expectedOutputMap, err := util.GetJsonMapFromJsonBytes(expectedOutputBytes)
	if err != nil {
		t.Fatalf("Failed to get json map from json bytes from expected output file %v with error: %v", expectedFileName, err)
	}
	isEqual := assert.True(t, reflect.DeepEqual(expectedOutputMap, resultMap))

	resultBytes, err := json.MarshalIndent(resultMap, "", "  ")
	assert.NoError(t, err)
	if !isEqual {
		log.Printf("Test %v %v failed: expectedMap=\n%v\nresultMap=\n%v", testData.testId, testData.testName, string(expectedOutputBytes), string(resultBytes))
	}
}

func shouldFail(t *testing.T, testData TestData) {
	if r := recover(); r != nil {
		if val, ok := r.(string); ok {
			fmt.Println(val)
		}
		for _, errMessage := range translator.ErrorMessages {
			fmt.Println(errMessage)
		}
		if !testData.shouldFail {
			assert.Fail(t, fmt.Sprintf("Test %v %v should not have failures.", testData.testId, testData.testName))
		}
	} else {
		if testData.shouldFail {
			assert.Fail(t, fmt.Sprintf("Test %v %v should have failures.", testData.testId, testData.testName))
		}
	}
	log.Printf("Test %v %v finished", testData.testId, testData.testName)
}
