// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func checkResult(t *testing.T, inputBytes []byte, expectedOutput interface{}) {
	p := new(Procstat)
	var input interface{}
	if e := json.Unmarshal(inputBytes, &input); e == nil {
		_, actualOutput := p.ApplyRule(input)
		assert.Equal(t, expectedOutput, actualOutput, "Expect to be equal")
	} else {
		panic(e)
	}
}

//Check the case when the input is in "procstat":{//specific configuration}
func TestExeConfig(t *testing.T) {
	input := []byte(`{"procstat": [
	{
	    "measurement": [
		{"name": "cpu_usage", "rename": "cwagent_cpu_usage", "unit": "Percent"},
		{"name": "memory_rss", "rename": "cwagent_mem_usage", "unit": "Bytes"}
	    ],
	    "exe": "amazon-cloudwat"
	}
      ]}`)
	expectedVal := []interface{}{map[string]interface{}{
		"exe":        "amazon-cloudwat",
		"pid_finder": "native",
		"fieldpass":  []string{"cpu_usage", "memory_rss"},
		"tagexclude": []string{"user", "result"},
	}}
	checkResult(t, input, expectedVal)
}

func TestPidFileConfig(t *testing.T) {
	input := []byte(`{"procstat": [
	{
	    "measurement": [
		{"name": "cpu_usage", "rename": "cwagent_cpu_usage", "unit": "Percent"},
		{"name": "memory_rss", "rename": "cwagent_mem_usage", "unit": "Bytes"}
	    ],
	    "pid_file": "/var/run/sshd"
	}
      ]}`)
	expectedVal := []interface{}{map[string]interface{}{
		"pid_file":   "/var/run/sshd",
		"pid_finder": "native",
		"fieldpass":  []string{"cpu_usage", "memory_rss"},
		"tagexclude": []string{"user", "result"},
	}}
	checkResult(t, input, expectedVal)
}

func TestPatternConfig(t *testing.T) {
	input := []byte(`{"procstat": [
	{
	    "measurement": [
		{"name": "cpu_usage", "rename": "cwagent_cpu_usage", "unit": "Percent"},
		{"name": "memory_rss", "rename": "cwagent_mem_usage", "unit": "Bytes"}
	    ],
	    "pattern": "sshd"
	}
      ]}`)
	expectedVal := []interface{}{map[string]interface{}{
		"pattern":    "sshd",
		"pid_finder": "native",
		"fieldpass":  []string{"cpu_usage", "memory_rss"},
		"tagexclude": []string{"user", "result"},
	}}
	checkResult(t, input, expectedVal)
}

func TestMultiLookupConfig(t *testing.T) {
	input := []byte(`{"procstat": [
	{
	    "measurement": [
		{"name": "cpu_usage", "rename": "cwagent_cpu_usage", "unit": "Percent"},
		{"name": "memory_rss", "rename": "cwagent_mem_usage", "unit": "Bytes"}
	    ],
	    "pid_file": "/var/run/sshd",
	    "exe": "cloudwatch",
	    "pattern": "sshd"
	}
      ]}`)
	expectedVal := []interface{}{map[string]interface{}{
		"pid_file":   "/var/run/sshd",
		"exe":        "cloudwatch",
		"pattern":    "sshd",
		"pid_finder": "native",
		"fieldpass":  []string{"cpu_usage", "memory_rss"},
		"tagexclude": []string{"user", "result"},
	}}
	checkResult(t, input, expectedVal)
}

func TestIntervalConfig(t *testing.T) {
	input := []byte(`{"procstat": [
	{
	    "measurement": [
		{"name": "cpu_usage", "rename": "cwagent_cpu_usage", "unit": "Percent"},
		{"name": "memory_rss", "rename": "cwagent_mem_usage", "unit": "Bytes"}
	    ],
	    "metrics_collection_interval": 30,
	    "pid_file": "/var/run/sshd"
	}
      ]}`)
	expectedVal := []interface{}{map[string]interface{}{
		"pid_file":   "/var/run/sshd",
		"pid_finder": "native",
		"fieldpass":  []string{"cpu_usage", "memory_rss"},
		"interval":   "30s",
		"tags":       map[string]interface{}{"aws:StorageResolution": "true"},
		"tagexclude": []string{"user", "result"},
	}}
	checkResult(t, input, expectedVal)
}

func TestMultiProcessesConfig(t *testing.T) {
	input := []byte(`{"procstat": [
	{
	    "measurement": [
		{"name": "cpu_usage", "rename": "cwagent_cpu_usage", "unit": "Percent"},
		{"name": "memory_rss", "rename": "cwagent_mem_usage", "unit": "Bytes"}
	    ],
	    "pid_file": "/var/run/sshd"
	},
	{
	    "measurement": [
		{"name": "cpu_usage", "rename": "cwagent_cpu_usage", "unit": "Percent"},
		{"name": "memory_rss", "rename": "cwagent_mem_usage", "unit": "Bytes"}
	    ],
	    "exe": "cloudwatch"
	}
      ]}`)
	expectedVal := []interface{}{
		map[string]interface{}{
			"pid_file":   "/var/run/sshd",
			"pid_finder": "native",
			"fieldpass":  []string{"cpu_usage", "memory_rss"},
			"tagexclude": []string{"user", "result"},
		},
		map[string]interface{}{
			"exe":        "cloudwatch",
			"pid_finder": "native",
			"fieldpass":  []string{"cpu_usage", "memory_rss"},
			"tagexclude": []string{"user", "result"},
		},
	}
	checkResult(t, input, expectedVal)
}

func TestMeasurementMissingConfig(t *testing.T) {
	input := []byte(`{"procstat": [
	{
	    "metrics_collection_interval": 30,
	    "pid_file": "/var/run/sshd"
	}
      ]}`)
	checkResult(t, input, "")
}

func TestIntervalErrorConfig(t *testing.T) {
	input := []byte(`{"procstat": [
	{
	    "measurement": ["cpu_usage"],
	    "metrics_collection_interval": "30s",
	    "pid_file": "/var/run/sshd"
	}
      ]}`)
	expectedVal := []interface{}{map[string]interface{}{
		"pid_file":   "/var/run/sshd",
		"pid_finder": "native",
		"fieldpass":  []string{"cpu_usage"},
		"tagexclude": []string{"user", "result"},
	}}
	checkResult(t, input, expectedVal)
}
