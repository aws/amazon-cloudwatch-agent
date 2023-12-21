// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

func checkIfSchemaValidateAsExpected(t *testing.T, jsonInputPath string, shouldSuccess bool, expectedErrorMap map[string]int) {
	actualErrorMap := make(map[string]int)

	jsonInputMap, err := util.GetJsonMapFromFile(jsonInputPath)
	if err != nil {
		t.Fatalf("Failed to get json map from %v with error: %v", jsonInputPath, err)
	}

	result, err := cmdutil.RunSchemaValidation(jsonInputMap)
	if err != nil {
		t.Fatalf("Failed to run schema validation: %v", err)
	}

	if result.Valid() {
		assert.True(t, shouldSuccess, "It should fail the schemaValidation!")
	} else {
		errorDetails := result.Errors()
		for _, errorDetail := range errorDetails {
			t.Logf("String: %v \n", errorDetail.String())
			t.Logf("Context: %v \n", errorDetail.Context().String())
			t.Logf("Description: %v \n", errorDetail.Description())
			t.Logf("Details: %v \n", errorDetail.Details())
			t.Logf("Field: %v \n", errorDetail.Field())
			t.Logf("Type: %v \n", errorDetail.Type())
			t.Logf("Value: %v \n", errorDetail.Value())
			if _, ok := actualErrorMap[errorDetail.Type()]; ok {
				actualErrorMap[errorDetail.Type()] += 1
			} else {
				actualErrorMap[errorDetail.Type()] = 1
			}
		}
		assert.Equal(t, expectedErrorMap, actualErrorMap, "Unexpected error set!")
		assert.False(t, shouldSuccess, "It should pass the schemaValidation!")
	}

}

func TestAgentConfig(t *testing.T) {
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validAgent.json", true, map[string]int{})
	expectedErrorMap := map[string]int{}
	expectedErrorMap["invalid_type"] = 5
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidAgent.json", false, expectedErrorMap)
}

func TestTracesConfig(t *testing.T) {
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validTrace.json", true, map[string]int{})
	expectedErrorMap := map[string]int{}
	expectedErrorMap["array_min_properties"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidTrace.json", false, expectedErrorMap)
}

func TestLogFilesConfig(t *testing.T) {
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validLogFiles.json", true, map[string]int{})
	expectedErrorMap := map[string]int{}
	expectedErrorMap["array_min_items"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidLogFilesWithNoFileConfigured.json", false, expectedErrorMap)
	expectedErrorMap1 := map[string]int{}
	expectedErrorMap1["required"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidLogFilesWithMissingFilePath.json", false, expectedErrorMap1)
	expectedErrorMap2 := map[string]int{}
	expectedErrorMap2["unique"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidLogFilesWithDuplicateEntry.json", false, expectedErrorMap2)
}

func TestLogWindowsEventConfig(t *testing.T) {
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validLogWindowsEvents.json", true, map[string]int{})
	expectedErrorMap := map[string]int{}
	expectedErrorMap["number_not"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidLogWindowsEventsWithInvalidEventName.json", false, expectedErrorMap)
	expectedErrorMap1 := map[string]int{}
	expectedErrorMap1["required"] = 2
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidLogWindowsEventsWithMissingEventNameAndLevel.json", false, expectedErrorMap1)
	expectedErrorMap2 := map[string]int{}
	expectedErrorMap2["invalid_type"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidLogWindowsEventsWithInvalidEventLevelType.json", false, expectedErrorMap2)
	expectedErrorMap3 := map[string]int{}
	expectedErrorMap3["enum"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidLogWindowsEventsWithInvalidEventFormatType.json", false, expectedErrorMap3)
}

func TestMetricsConfig(t *testing.T) {
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validLinuxMetrics.json", true, map[string]int{})
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validWindowsMetrics.json", true, map[string]int{})
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validMetricsWithAppSignals.json", true, map[string]int{})
	expectedErrorMap := map[string]int{}
	expectedErrorMap["invalid_type"] = 2
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidMetricsWithInvalidAggregationDimensions.json", false, expectedErrorMap)
	expectedErrorMap1 := map[string]int{}
	expectedErrorMap1["array_min_properties"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidMetricsWithNoMetricsDefined.json", false, expectedErrorMap1)
	expectedErrorMap2 := map[string]int{}
	expectedErrorMap2["required"] = 1
	expectedErrorMap2["invalid_type"] = 2
	expectedErrorMap2["number_one_of"] = 2
	expectedErrorMap2["number_all_of"] = 3
	expectedErrorMap2["unique"] = 1
	expectedErrorMap2["number_gte"] = 1
	expectedErrorMap2["string_gte"] = 2
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidMetricsWithInvalidMeasurement.json", false, expectedErrorMap2)
	expectedErrorMap3 := map[string]int{}
	expectedErrorMap3["invalid_type"] = 2
	expectedErrorMap3["number_all_of"] = 2
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidMetricsWithInvalidAppendDimensions.json", false, expectedErrorMap3)
	expectedErrorMap4 := map[string]int{}
	expectedErrorMap4["enum"] = 1
	expectedErrorMap4["array_max_items"] = 1
	expectedErrorMap4["invalid_type"] = 1
	expectedErrorMap4["number_all_of"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidMetricsWithinvalidMetricsCollected.json", false, expectedErrorMap4)
	expectedErrorMap5 := map[string]int{}
	expectedErrorMap5["additional_property_not_allowed"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidMetricsWithAdditionalProperties.json", false, expectedErrorMap5)
	expectedErrorMap6 := map[string]int{}
	expectedErrorMap6["required"] = 1
	expectedErrorMap6["invalid_type"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidMetricsWithInvalidMetrics_Collected.json", false, expectedErrorMap6)
}

func TestProcstatConfig(t *testing.T) {
	expectedErrorMap := map[string]int{}
	expectedErrorMap["invalid_type"] = 1
	expectedErrorMap["number_all_of"] = 1
	expectedErrorMap["number_any_of"] = 1
	expectedErrorMap["required"] = 1
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidProcstatMeasurement.json", false, expectedErrorMap)

	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validProcstatConfig.json", true, map[string]int{})
}

func TestEthtoolConfig(t *testing.T) {
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validEthtoolConfig.json", true, map[string]int{})
}

func TestNvidiaGpuConfig(t *testing.T) {
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validNvidiaGpuConfig.json", true, map[string]int{})
}

func TestValidLogFilterConfig(t *testing.T) {
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/validLogFilesWithFilters.json", true, map[string]int{})
}

func TestInvalidLogFilterConfig(t *testing.T) {
	expectedErrorMap := map[string]int{
		"additional_property_not_allowed": 1,
		"enum":                            1,
	}
	checkIfSchemaValidateAsExpected(t, "../../translator/config/sampleSchema/invalidLogFilesWithFilters.json", false, expectedErrorMap)
}

// Validate all sampleConfig files schema
func TestSampleConfigSchema(t *testing.T) {
	if files, err := os.ReadDir("../../translator/tocwconfig/sampleConfig/"); err == nil {
		re := regexp.MustCompile(".json")
		for _, file := range files {
			if re.MatchString(file.Name()) {
				t.Logf("Validating ../../translator/tocwconfig/sampleConfig/%s\n", file.Name())
				checkIfSchemaValidateAsExpected(t, "../../translator/tocwconfig/sampleConfig/"+file.Name(), true, map[string]int{})
				t.Logf("Validated ../../translator/tocwconfig/sampleConfig/%s\n", file.Name())
			}
		}
	} else {
		panic(err)
	}
}
