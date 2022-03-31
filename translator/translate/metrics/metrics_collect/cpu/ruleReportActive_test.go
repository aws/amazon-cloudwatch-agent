// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cpu

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportActive_NoActive(t *testing.T) {
	r := new(ReportActive)
	var input interface{}
	err := json.Unmarshal([]byte(`{
			"measurement":[
				"cpu_usage_idle"
			]
	}`), &input)
	if err == nil {
		actualReturnKey, _ := r.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey)
	} else {
		panic(err)
	}
}

func TestReportActive_TimeActive(t *testing.T) {
	r := new(ReportActive)
	var input interface{}
	err := json.Unmarshal([]byte(`{
			"measurement": [
				"cpu_usage_idle",
				"cpu_time_active"
			]
	}`), &input)
	if err == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "report_active", actualReturnKey)
		assert.True(t, actualReturnValue.(bool))
	} else {
		panic(err)
	}
}

func TestReportActive_UsageActive(t *testing.T) {
	r := new(ReportActive)
	var input interface{}
	err := json.Unmarshal([]byte(`{
			"measurement": [
				"cpu_usage_idle",
				"usage_active"
			]
	}`), &input)
	if err == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "report_active", actualReturnKey)
		assert.True(t, actualReturnValue.(bool))
	} else {
		panic(err)
	}
}
