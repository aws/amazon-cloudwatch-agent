// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testSection = "test"
)

func getSectionWithoutTags() map[string]interface{} {
	input := map[string]interface{}{}
	input["fieldpass"] = "fp1"

	output := map[string]interface{}{}
	output["namespace"] = "ns1"

	processor := map[string]interface{}{}
	processor["refresh_interval_seconds"] = "0"

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor}},
	}
}

func getExpectedSectionWithoutTags() map[string]interface{} {
	input := map[string]interface{}{}
	input["fieldpass"] = "fp1"
	input["tags"] = map[string]interface{}{routingTagKey: testSection}

	output := map[string]interface{}{}
	output["namespace"] = "ns1"
	output["tagexclude"] = []string{routingTagKey}
	output["tagpass"] = map[string][]string{routingTagKey: {testSection}}

	processor := map[string]interface{}{}
	processor["refresh_interval_seconds"] = "0"
	processor["tagpass"] = map[string][]string{routingTagKey: {testSection}}

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor}},
	}
}

func getSectionWithTags() map[string]interface{} {
	input := map[string]interface{}{}
	input["tags"] = map[string]interface{}{"someTag": "anyValue"}
	input["fieldpass"] = "fp1"

	output := map[string]interface{}{}
	output["namespace"] = "ns1"
	output["tagpass"] = map[string][]string{routingTagKey: {"anyValue"}}
	output["tagexclude"] = []string{"someTag"}

	processor := map[string]interface{}{}
	processor["refresh_interval_seconds"] = "0"
	processor["tagpass"] = map[string][]string{routingTagKey: {"anyValue"}}

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor}},
	}
}

func getExpectedSectionWithTags() map[string]interface{} {
	input := map[string]interface{}{}
	input["tags"] = map[string]interface{}{"someTag": "anyValue", routingTagKey: testSection}
	input["fieldpass"] = "fp1"

	output := map[string]interface{}{}
	output["namespace"] = "ns1"
	output["tagpass"] = map[string][]string{routingTagKey: {"anyValue", testSection}}
	output["tagexclude"] = []string{"someTag", routingTagKey}

	processor := map[string]interface{}{}
	processor["refresh_interval_seconds"] = "0"
	processor["tagpass"] = map[string][]string{routingTagKey: {"anyValue", testSection}}

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor}},
	}
}

func getSectionWithTagsMetricPath() map[string]interface{} {
	input := map[string]interface{}{}
	input["tags"] = map[string]interface{}{routingTagKey: "metrics"}
	input["fieldpass"] = "fp1"

	output := map[string]interface{}{}
	output["namespace"] = "ns1"
	output["tagpass"] = map[string][]string{routingTagKey: {"anyValue"}}
	output["tagexclude"] = []string{"someTag"}

	processor := map[string]interface{}{}
	processor["refresh_interval_seconds"] = "0"
	processor["tagpass"] = map[string][]string{routingTagKey: {"anyValue"}}

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor}},
	}
}

func getExpectedSectionWithTagsMetricPath() map[string]interface{} {
	input := map[string]interface{}{}
	input["tags"] = map[string]interface{}{routingTagKey: "metrics"}
	input["fieldpass"] = "fp1"

	output := map[string]interface{}{}
	output["namespace"] = "ns1"
	output["tagpass"] = map[string][]string{routingTagKey: {"anyValue", testSection}}
	output["tagexclude"] = []string{"someTag", routingTagKey}

	processor := map[string]interface{}{}
	processor["refresh_interval_seconds"] = "0"
	processor["tagpass"] = map[string][]string{routingTagKey: {"anyValue", testSection}}

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor}},
	}
}

func getExpectedSectionOneInputWithoutTags() map[string]interface{} {
	routingTagVal := testSection + linkedCharacter + "mem"
	input := map[string]interface{}{}
	input["tags"] = map[string]interface{}{routingTagKey: routingTagVal}
	input["fieldpass"] = "fp1"

	output := map[string]interface{}{}
	output["namespace"] = "ns1"
	output["tagpass"] = map[string][]string{routingTagKey: {routingTagVal}}
	output["tagexclude"] = []string{routingTagKey}

	processor := map[string]interface{}{}
	processor["refresh_interval_seconds"] = "0"
	processor["tagpass"] = map[string][]string{routingTagKey: {routingTagVal}}

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor}},
	}
}

func getExpectedSectionOneInputWithTags() map[string]interface{} {
	routingTagVal := testSection + linkedCharacter + "mem"
	input := map[string]interface{}{}
	input["tags"] = map[string]interface{}{"someTag": "anyValue", routingTagKey: routingTagVal}
	input["fieldpass"] = "fp1"

	output := map[string]interface{}{}
	output["namespace"] = "ns1"
	output["tagpass"] = map[string][]string{routingTagKey: {"anyValue", routingTagVal}}
	output["tagexclude"] = []string{"someTag", routingTagKey}

	processor := map[string]interface{}{}
	processor["refresh_interval_seconds"] = "0"
	processor["tagpass"] = map[string][]string{routingTagKey: {"anyValue", routingTagVal}}

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor}},
	}
}

func getSectionWithTwoInputs() map[string]interface{} {
	input1 := map[string]interface{}{}
	input1["fieldpass"] = "fp1"

	input2 := map[string]interface{}{}
	input2["fieldpass"] = "fp2"

	output := map[string]interface{}{}
	output["namespace"] = "ns1"

	processor1 := map[string]interface{}{}
	processor1["refresh_interval_seconds"] = "0"

	processor2 := map[string]interface{}{}
	processor2["refresh_interval_seconds"] = "0"

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input1}, "cpu": []interface{}{input2}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor1}, "k8sdecorator": []interface{}{processor2}},
	}
}

func getExpectedSectionWithTwoInputs() map[string]interface{} {
	routingTagVal := testSection + linkedCharacter + "mem"

	input1 := map[string]interface{}{}
	input1["fieldpass"] = "fp1"
	input1["tags"] = map[string]interface{}{routingTagKey: routingTagVal}

	input2 := map[string]interface{}{}
	input2["fieldpass"] = "fp2"
	input2["tags"] = map[string]interface{}{routingTagKey: testSection}

	output := map[string]interface{}{}
	output["namespace"] = "ns1"
	output["tagexclude"] = []string{routingTagKey}
	output["tagpass"] = map[string][]string{routingTagKey: {testSection, routingTagVal}}

	processor1 := map[string]interface{}{}
	processor1["refresh_interval_seconds"] = "0"
	processor1["tagpass"] = map[string][]string{routingTagKey: {testSection}}

	processor2 := map[string]interface{}{}
	processor2["refresh_interval_seconds"] = "0"
	processor2["tagpass"] = map[string][]string{routingTagKey: {testSection, routingTagVal}}

	return map[string]interface{}{
		"inputs":     map[string]interface{}{"mem": []interface{}{input1}, "cpu": []interface{}{input2}},
		"outputs":    map[string]interface{}{"cloudwatch": []interface{}{output}},
		"processors": map[string]interface{}{"ec2tagger": []interface{}{processor1}, "k8sdecorator": []interface{}{processor2}},
	}
}

func TestSetMetricPath_NoTags(t *testing.T) {
	actual := getSectionWithoutTags()
	expected := getExpectedSectionWithoutTags()

	SetMetricPath(actual, testSection)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestSetMetricPath_Tags(t *testing.T) {
	actual := getSectionWithTags()
	expected := getExpectedSectionWithTags()

	SetMetricPath(actual, testSection)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestSetMetricPath_TagsWithMetricPath(t *testing.T) {
	actual := getSectionWithTagsMetricPath()
	expected := getExpectedSectionWithTagsMetricPath()

	SetMetricPath(actual, testSection)
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestSetMetricPathForOneInput_NoTags(t *testing.T) {
	actual := getSectionWithoutTags()
	expected := getExpectedSectionOneInputWithoutTags()

	SetMetricPathForOneInput(actual, testSection, "mem", []string{"ec2tagger"})
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestSetMetricPathForOneInput_Tags(t *testing.T) {
	actual := getSectionWithTags()
	expected := getExpectedSectionOneInputWithTags()

	SetMetricPathForOneInput(actual, testSection, "mem", []string{"ec2tagger"})
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestSetMetricPathWithOnePathOverride(t *testing.T) {
	actual := getSectionWithTwoInputs()
	expected := getExpectedSectionWithTwoInputs()

	SetMetricPath(actual, testSection)
	//override processor plugins for one input plugin
	SetMetricPathForOneInput(actual, testSection, "mem", []string{"k8sdecorator"})
	assert.Equal(t, expected, actual, "Expected to be equal")
}
