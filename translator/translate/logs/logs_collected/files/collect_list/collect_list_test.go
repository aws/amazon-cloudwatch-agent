// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestFileConfig(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{"collect_list":[{"file_path":"path1",
            "log_group_name":"group1","log_stream_name":"LOG_STREAM_NAME", "log_group_class":"STANDARD"}]}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)

	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    true,
		"log_group_name":    "group1",
		"log_stream_name":   "LOG_STREAM_NAME",
		"log_group_class":   util.StandardLogGroupClass,
		"pipe":              false,
		"retention_in_days": -1,
	}}
	assert.Equal(t, expectVal, val)
}

func TestFileConfigOverride(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{"collect_list":[{"file_path":"path1",
            "log_group_name":"group1","from_beginning":false}]}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    false,
		"log_group_name":    "group1",
		"pipe":              false,
		"retention_in_days": -1,
		"log_group_class":   "",
	}}
	assert.Equal(t, expectVal, val)
}

func TestTimestampFormat(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"timestamp_format":"%H:%M:%S %y %b %-d",
				"timezone":"UTC"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    true,
		"pipe":              false,
		"timestamp_layout":  []string{"15:04:05 06 Jan _2"},
		"timestamp_regex":   "(\\d{2}:\\d{2}:\\d{2} \\d{2} \\w{3} \\s{0,1}\\d{1,2})",
		"timezone":          "UTC",
		"retention_in_days": -1,
		"log_group_class":   "",
	}}
	assert.Equal(t, expectVal, val)
}

func TestTimestampFormatAll(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			input: `{
					"collect_list":[
						{
							"file_path":"path1",
							"timestamp_format":"%H:%M:%S %y %b %-d"
						}
					]
				}`,
			expected: []interface{}{map[string]interface{}{
				"file_path":         "path1",
				"from_beginning":    true,
				"pipe":              false,
				"retention_in_days": -1,
				"timestamp_layout":  []string{"15:04:05 06 Jan _2"},
				"timestamp_regex":   "(\\d{2}:\\d{2}:\\d{2} \\d{2} \\w{3} \\s{0,1}\\d{1,2})",
				"log_group_class":   "",
			}},
		},
		{
			input: `{
					"collect_list":[
						{
							"file_path":"path1",
							"timestamp_format":"%-m %-d %H:%M:%S"
						}
					]
				}`,
			expected: []interface{}{map[string]interface{}{
				"file_path":         "path1",
				"from_beginning":    true,
				"pipe":              false,
				"retention_in_days": -1,
				"timestamp_layout":  []string{"1 _2 15:04:05", "01 _2 15:04:05"},
				"timestamp_regex":   "(\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
				"log_group_class":   "",
			}},
		},
		{
			input: `{
					"collect_list":[
						{
							"file_path":"path1",
							"timestamp_format":"%-d %-m %H:%M:%S"
						}
					]
				}`,
			expected: []interface{}{map[string]interface{}{
				"file_path":         "path1",
				"from_beginning":    true,
				"pipe":              false,
				"retention_in_days": -1,
				"timestamp_layout":  []string{"_2 1 15:04:05", "_2 01 15:04:05"},
				"timestamp_regex":   "(\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
				"log_group_class":   "",
			}},
		},
		{
			input: `{
					"collect_list":[
						{
							"file_path":"path4",
                            "timestamp_format": "%b %d %H:%M:%S"
						}
					]
				}`,
			expected: []interface{}{map[string]interface{}{
				"file_path":         "path4",
				"from_beginning":    true,
				"pipe":              false,
				"retention_in_days": -1,
				"timestamp_layout":  []string{"Jan _2 15:04:05"},
				"timestamp_regex":   "(\\w{3} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
				"log_group_class":   "",
			}},
		},
		{
			input: `{
					"collect_list":[
						{
							"file_path":"path5",
                            "timestamp_format": "%b %-d %H:%M:%S"
						}
					]
				}`,
			expected: []interface{}{map[string]interface{}{
				"file_path":         "path5",
				"from_beginning":    true,
				"pipe":              false,
				"retention_in_days": -1,
				"timestamp_layout":  []string{"Jan _2 15:04:05"},
				"timestamp_regex":   "(\\w{3} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
				"log_group_class":   "",
			}},
		},
		{
			input: `{
					"collect_list":[
						{
							"file_path":"path1",
							"timestamp_format":"%-S %-d %-m %H:%M:%S"
						}
					]
				}`,
			expected: []interface{}{map[string]interface{}{
				"file_path":         "path1",
				"from_beginning":    true,
				"pipe":              false,
				"retention_in_days": -1,
				"timestamp_layout":  []string{"5 _2 1 15:04:05", "5 _2 01 15:04:05"},
				"timestamp_regex":   "(\\d{1,2} \\s{0,1}\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
				"log_group_class":   "",
			}},
		},
		{
			input: `{
					"collect_list":[
						{
							"file_path":"path7",
							"timestamp_format":"%-S %-d %m %H:%M:%S"
						}
					]
				}`,
			expected: []interface{}{map[string]interface{}{
				"file_path":         "path7",
				"from_beginning":    true,
				"pipe":              false,
				"retention_in_days": -1,
				"timestamp_layout":  []string{"5 _2 01 15:04:05", "5 _2 1 15:04:05"},
				"timestamp_regex":   "(\\d{1,2} \\s{0,1}\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
				"log_group_class":   "",
			}},
		},
	}

	for _, tt := range tests {
		result := applyRule1(t, tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func applyRule1(t *testing.T, buf string) interface{} {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(buf), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	return val
}

// -hour:-minute:-seconds does not work for golang parser.
func TestTimestampFormat_NonZeroPadding(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"timestamp_format":"%-I:%-M:%-S %y %-m %-d",
				"timezone":"UTC"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectedLayout := []string{"3:4:5 06 1 _2", "3:4:5 06 01 _2"}
	expectedRegex := "(\\d{1,2}:\\d{1,2}:\\d{1,2} \\d{2} \\s{0,1}\\d{1,2} \\s{0,1}\\d{1,2})"
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_class":   "",
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"timestamp_layout":  expectedLayout,
		"timestamp_regex":   expectedRegex,
		"timezone":          "UTC",
	}}
	assert.Equal(t, expectVal, val)

	sampleLogEntries := []string{
		"1:2:3 18 3 8 - Log Content",
		"1:2:3 18  3  8 - Log Content",
	}
	for _, sampleLogEntry := range sampleLogEntries {
		regex := regexp.MustCompile(expectedRegex)
		match := regex.FindStringSubmatch(sampleLogEntry)
		assert.NotNil(t, match)
		assert.Equal(t, 2, len(match))
		parsedTime, err := time.ParseInLocation(expectedLayout[0], match[1], time.UTC)
		assert.NoError(t, err)
		assert.Equal(t, time.Date(2018, 3, 8, 1, 2, 3, 0, time.UTC), parsedTime)
	}
}

// ^ . * ? | [ ] ( ) { } $
func TestTimestampFormat_SpecialCharacters(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"timestamp_format":"^.*?|[({%H:%M:%S %y %b %-d})]$",
				"timezone":"UTC"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectedLayout := []string{"^.*?|[({15:04:05 06 Jan _2})]$"}
	expectedRegex := "(\\^\\.\\*\\?\\|\\[\\(\\{\\d{2}:\\d{2}:\\d{2} \\d{2} \\w{3} \\s{0,1}\\d{1,2}\\}\\)\\]\\$)"
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_class":   "",
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"timestamp_layout":  expectedLayout,
		"timestamp_regex":   expectedRegex,
		"timezone":          "UTC",
	}}
	assert.Equal(t, expectVal, val)

	sampleLogEntry := "^.*?|[({12:52:00 17 Dec 27})]$ Log Content"
	regex := regexp.MustCompile(expectedRegex)
	match := regex.FindStringSubmatch(sampleLogEntry)
	assert.NotNil(t, match)
	assert.Equal(t, 2, len(match))

	parsedTime, err := time.ParseInLocation(expectedLayout[0], match[1], time.UTC)
	assert.NoError(t, err)
	assert.Equal(t, time.Date(2017, 12, 27, 12, 52, 0, 0, time.UTC), parsedTime)
}

func TestTimestampFormat_Template(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"timestamp_format":"%b %-d %H:%M:%S"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectedLayout := []string{"Jan _2 15:04:05"}
	expectedRegex := "(\\w{3} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})"
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_class":   "",
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"timestamp_layout":  expectedLayout,
		"timestamp_regex":   expectedRegex,
	}}
	assert.Equal(t, expectVal, val)

	sampleLogEntry := "Aug  9 20:45:51 - Log Jun 18 22:33:07 Content"
	regex := regexp.MustCompile(expectedRegex)
	match := regex.FindStringSubmatch(sampleLogEntry)
	assert.NotNil(t, match)
	assert.Equal(t, 2, len(match))

	parsedTime, err := time.ParseInLocation(expectedLayout[0], match[1], time.Local)
	assert.NoError(t, err)
	assert.Equal(t, time.Date(0, 8, 9, 20, 45, 51, 0, time.Local), parsedTime)
}

func TestTimestampFormat_InvalidRegex(t *testing.T) {
	translator.ResetMessages()
	r := new(TimestampRegex)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"timestamp_format":"%Y-%m-%dT%H:%M%S+00:00"
	}`), &input)
	assert.Nil(t, e)

	retKey, retVal := r.ApplyRule(input)
	assert.Equal(t, "timestamp_regex", retKey)
	assert.Nil(t, retVal)
	assert.Len(t, translator.ErrorMessages, 1)

}

func TestMultiLineStartPattern(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"timestamp_format":"%H:%M:%S %y %b %d",
				"timezone":"UTC",
				"multi_line_start_pattern":"{timestamp_format}"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":                "path1",
		"from_beginning":           true,
		"pipe":                     false,
		"retention_in_days":        -1,
		"log_group_class":          "",
		"timestamp_layout":         []string{"15:04:05 06 Jan _2"},
		"timestamp_regex":          "(\\d{2}:\\d{2}:\\d{2} \\d{2} \\w{3} \\s{0,1}\\d{1,2})",
		"timezone":                 "UTC",
		"multi_line_start_pattern": "{timestamp_regex}",
	}}
	assert.Equal(t, expectVal, val)
}

func TestEncoding(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"timestamp_format":"%H:%M:%S %y %b %d",
				"timezone":"UTC",
				"encoding":"gbk"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"log_group_class":   "",
		"timestamp_layout":  []string{"15:04:05 06 Jan _2"},
		"timestamp_regex":   "(\\d{2}:\\d{2}:\\d{2} \\d{2} \\w{3} \\s{0,1}\\d{1,2})",
		"timezone":          "UTC",
		"encoding":          "gbk",
	}}
	assert.Equal(t, expectVal, val)
}

func TestEncoding_Invalid(t *testing.T) {
	translator.ResetMessages()
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
				"encoding":"xxx"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    true,
		"pipe":              false,
		"log_group_class":   "",
		"retention_in_days": -1,
	}}
	assert.Equal(t, expectVal, val)
	assert.False(t, translator.IsTranslateSuccess())
	assert.Equal(t, 1, len(translator.ErrorMessages))
	assert.Equal(t, "Under path : /logs/logs_collected/files/collect_list/encoding | Error : Encoding xxx is an invalid value.", translator.ErrorMessages[len(translator.ErrorMessages)-1])
}

func TestAutoRemoval(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
				"auto_removal": true
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"log_group_class":   "",
		"auto_removal":      true,
	}}
	assert.Equal(t, expectVal, val)

	e = json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
				"auto_removal": false
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val = f.ApplyRule(input)
	expectVal = []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"auto_removal":      false,
		"log_group_class":   "",
	}}
	assert.Equal(t, expectVal, val)

	e = json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val = f.ApplyRule(input)
	expectVal = []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"log_group_class":   "",
	}}
	assert.Equal(t, expectVal, val)
}

func TestFileConfigOutputFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	context.CurrentContext().SetOutputTomlFilePath(filepath.Join(dir, "amazon-cloudwatch-agent.toml"))
	agent.Global_Config.Region = "us-east-1"

	f := new(FileConfig)
	var input interface{}
	err = json.Unmarshal([]byte(`
		{"collect_list":[
			{
				"file_path":"path2",
				"log_group_name":"group2"
			},
			{
				"file_path":"path4",
				"log_group_name":"group2"
			},
			{
				"file_path":"path1",
				"log_group_name":"group1"
			},
			{
				"file_path":"path3",
				"log_group_name":"group3"
			}
		]}`), &input)
	assert.NoError(t, err)
	f.ApplyRule(input)

	path := filepath.Join(dir, logConfigOutputFileName)
	_, err = os.Stat(path)
	assert.NoError(t, err)

	bytes, err := os.ReadFile(path)
	assert.NoError(t, err)

	expectVal := "{\"version\":\"1\",\"log_configs\":[{\"log_group_name\":\"group1\"},{\"log_group_name\":\"group2\"},{\"log_group_name\":\"group3\"}],\"region\":\"us-east-1\"}"
	assert.Equal(t, expectVal, string(bytes))

	context.ResetContext()
	agent.Global_Config.Region = ""
}

func TestPublishMultiLogs_WithBlackList(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
				"blacklist": "^agent.log",
				"publish_multi_logs": true,
				"timezone": "UTC"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":          "path1",
		"from_beginning":     true,
		"pipe":               false,
		"retention_in_days":  -1,
		"log_group_class":    "",
		"blacklist":          "^agent.log",
		"publish_multi_logs": true,
		"timezone":           "UTC",
	}}
	assert.Equal(t, expectVal, val)

	e = json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
				"publish_multi_logs": false,
				"timezone": "UTC"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val = f.ApplyRule(input)
	expectVal = []interface{}{map[string]interface{}{
		"file_path":          "path1",
		"from_beginning":     true,
		"pipe":               false,
		"retention_in_days":  -1,
		"publish_multi_logs": false,
		"timezone":           "UTC",
		"log_group_class":    "",
	}}
	assert.Equal(t, expectVal, val)

	e = json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val = f.ApplyRule(input)
	expectVal = []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"log_group_class":   "",
	}}
	assert.Equal(t, expectVal, val)
}

func TestLogFilters(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"filters": [
					{"type": "include", "expression": "foo"},
					{"type": "exclude", "expression": "bar"}
				]
			}
		]
	}`), &input)
	assert.Nil(t, e)
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"from_beginning":    true,
		"pipe":              false,
		"retention_in_days": -1,
		"log_group_class":   "",
		"filters": []interface{}{
			map[string]interface{}{
				"type":       "include",
				"expression": "foo",
			},
			map[string]interface{}{
				"type":       "exclude",
				"expression": "bar",
			},
		},
	}}
	assert.Equal(t, expectVal, val)
}

func TestRetentionDifferentLogGroups(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"log_group_name":"test2",
				"retention_in_days":3
			},
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test2",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   "",
	}, map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   "",
	}}
	assert.Equal(t, expectVal, val)
}

func TestDuplicateRetention(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3
			},
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   "",
	}, map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   "",
	}}
	assert.Equal(t, expectVal, val)
}

func TestConflictingRetention(t *testing.T) {
	translator.ResetMessages()
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3
			},
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":5
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   "",
	}, map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 5,
		"from_beginning":    true,
		"log_group_class":   "",
	}}
	assert.Equal(t, "Under path : /logs/logs_collected/files/collect_list/ | Error : Different retention_in_days values can't be set for the same log group: test1", translator.ErrorMessages[len(translator.ErrorMessages)-1])
	assert.Equal(t, expectVal, val)
}

func TestDifferentLogGroupClasses(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"log_group_name":"test2",
				"retention_in_days":3
			},
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test2",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   "",
	}, map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   "",
	}}
	assert.Equal(t, expectVal, val)
}

func TestDuplicateLogGroupClass(t *testing.T) {
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3,
				"log_group_class": "standard"
			},
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3,
				"log_group_class": "standard"
			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   util.StandardLogGroupClass,
	}, map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   util.StandardLogGroupClass,
	}}
	assert.Equal(t, expectVal, val)
}

func TestConflictingLogGroupClass(t *testing.T) {
	translator.ResetMessages()
	f := new(FileConfig)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"collect_list":[
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3,
				"log_group_class":   "standard"

			},
			{
				"file_path":"path1",
       			"log_group_name":"test1",
				"retention_in_days":3,
				"log_group_class":   "Infrequent_access"

			}
		]
	}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := f.ApplyRule(input)
	expectVal := []interface{}{map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   util.StandardLogGroupClass,
	}, map[string]interface{}{
		"file_path":         "path1",
		"log_group_name":    "test1",
		"pipe":              false,
		"retention_in_days": 3,
		"from_beginning":    true,
		"log_group_class":   util.InfrequentAccessLogGroupClass,
	}}
	assert.Equal(t, "Under path : /logs/logs_collected/files/collect_list/ | Error : Different log_group_class values can't be set for the same log group: test1", translator.ErrorMessages[len(translator.ErrorMessages)-1])
	assert.Equal(t, expectVal, val)
}
