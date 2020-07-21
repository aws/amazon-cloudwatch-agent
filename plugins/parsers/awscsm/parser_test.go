// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	validJSON              = "{\"a\": 5, \"b\": {\"c\": 6}}"
	validJSONNewline       = "\n{\"d\": 7, \"b\": {\"d\": 8}}\n"
	validJSONArray         = "[{\"a\": 5, \"b\": {\"c\": 6}}]"
	validJSONArrayMultiple = "[{\"a\": 5, \"b\": {\"c\": 6}}, {\"a\": 7, \"b\": {\"c\": 8}}]"
	invalidJSON            = "I don't think this is JSON"
	invalidJSON2           = "{\"a\": 5, \"b\": \"c\": 6}}"
)

var (
	baseTagMap = map[string]string{
		"awscsm": "enabled",
	}
)

const validJSONTags = `
{
    "a": 5,
    "b": {
        "c": 6
    },
    "mytag": "foobar",
    "othertag": "baz"
}
`

const validJSONArrayTags = `
[
{
    "a": 5,
    "b": {
        "c": 6
    },
    "mytag": "foo",
    "othertag": "baz"
},
{
    "a": 7,
    "b": {
        "c": 8
    },
    "mytag": "bar",
    "othertag": "baz"
}
]
`

// Other than the excessive length test, these are
// appropriate variants of the base Json parser tests

func TestParseValidJSON(t *testing.T) {
	parser := JSONParser{
		MetricName: "json_test",
	}

	// Most basic vanilla test
	metrics, err := parser.Parse([]byte(validJSON))
	assert.NoError(t, err)
	assert.Len(t, metrics, 1)
	assert.Equal(t, "json_test", metrics[0].Name())
	assert.Equal(t, map[string]interface{}{
		"a":   float64(5),
		"b_c": float64(6),
	}, metrics[0].Fields())
	assert.Equal(t, baseTagMap, metrics[0].Tags())

	// Test that newlines are fine
	metrics, err = parser.Parse([]byte(validJSONNewline))
	assert.NoError(t, err)
	assert.Len(t, metrics, 1)
	assert.Equal(t, "json_test", metrics[0].Name())
	assert.Equal(t, map[string]interface{}{
		"d":   float64(7),
		"b_d": float64(8),
	}, metrics[0].Fields())
	assert.Equal(t, baseTagMap, metrics[0].Tags())

	// Test that strings without TagKeys defined are passed through
	metrics, err = parser.Parse([]byte(validJSONTags))
	assert.NoError(t, err)
	assert.Equal(t, "json_test", metrics[0].Name())
	assert.Equal(t, map[string]interface{}{
		"a":        float64(5),
		"b_c":      float64(6),
		"mytag":    "foobar",
		"othertag": "baz",
	}, metrics[0].Fields())
	assert.Equal(t, baseTagMap, metrics[0].Tags())

}

func TestParseLineValidJSON(t *testing.T) {
	parser := JSONParser{
		MetricName: "json_test",
	}

	// Most basic vanilla test
	metric, err := parser.ParseLine(validJSON)
	assert.NoError(t, err)
	assert.Equal(t, "json_test", metric.Name())
	assert.Equal(t, map[string]interface{}{
		"a":   float64(5),
		"b_c": float64(6),
	}, metric.Fields())
	assert.Equal(t, baseTagMap, metric.Tags())

	// Test that newlines are fine
	metric, err = parser.ParseLine(validJSONNewline)
	assert.NoError(t, err)
	assert.Equal(t, "json_test", metric.Name())
	assert.Equal(t, map[string]interface{}{
		"d":   float64(7),
		"b_d": float64(8),
	}, metric.Fields())
	assert.Equal(t, baseTagMap, metric.Tags())

	// Test that strings without TagKeys defined are passed through
	metric, err = parser.ParseLine(validJSONTags)
	assert.NoError(t, err)
	assert.Equal(t, "json_test", metric.Name())
	assert.Equal(t, map[string]interface{}{
		"a":        float64(5),
		"b_c":      float64(6),
		"mytag":    "foobar",
		"othertag": "baz",
	}, metric.Fields())
	assert.Equal(t, baseTagMap, metric.Tags())
}

func TestParseInvalidJSON(t *testing.T) {
	parser := JSONParser{
		MetricName: "json_test",
	}

	_, err := parser.Parse([]byte(invalidJSON))
	assert.Error(t, err)
	_, err = parser.Parse([]byte(invalidJSON2))
	assert.Error(t, err)
	_, err = parser.ParseLine(invalidJSON)
	assert.Error(t, err)
}

func buildSimpleJson(length int) bytes.Buffer {
	var jsonBuf bytes.Buffer
	jsonBuf.WriteString("{\"a\":\"")
	for i := 0; i < length; i++ {
		jsonBuf.WriteString("a")
	}
	jsonBuf.WriteString("\"}")

	return jsonBuf
}

func TestExcessiveSize(t *testing.T) {
	parser := JSONParser{
		MetricName: "json_test",
	}

	shortJson := buildSimpleJson(1000)
	_, err := parser.Parse(shortJson.Bytes())
	assert.NoError(t, err)

	longJSON := buildSimpleJson(10000)
	_, err = parser.Parse(longJSON.Bytes())
	assert.Error(t, err)
}
