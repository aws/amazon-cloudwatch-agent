// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"encoding/json"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/internal/csm"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"

	"github.com/stretchr/testify/assert"
)

const (
	overrideMemoryLimit = 10
	overridePort        = 2000
)

func TestCsm_Defaults(t *testing.T) {
	c := new(Csm)
	agent.Global_Config.Region = "us-east-1"

	var input interface{}
	err := json.Unmarshal([]byte(`{"csm":{}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	_, actual := c.ApplyRule(input)
	expected := map[string]interface{}{
		"outputs": map[string]interface{}{
			ConfOutputPluginKey: []interface{}{
				map[string]interface{}{
					agent.RegionKey:        "us-east-1",
					csm.MemoryLimitInMbKey: csm.DefaultMemoryLimitInMb,
					csm.LogLevelKey:        csm.DefaultLogLevel,
				},
			},
		},
		"inputs": map[string]interface{}{
			ConfInputPluginKey: []interface{}{
				map[string]interface{}{
					ConfInputAddressKey: []string{
						computeIPv4LoopbackAddressFromPort(csm.DefaultPort),
						computeIPv6LoopbackAddressFromPort(csm.DefaultPort),
					},
					csm.DataFormatKey: "aws_csm",
				},
			},
		},
	}

	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestCsm_Overrides(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"

	cases := map[string]struct {
		Input  string
		Expect map[string]interface{}
	}{
		"log_level": {
			Input: `{"csm":{"log_level":1}}`,
			Expect: map[string]interface{}{
				"outputs": map[string]interface{}{
					ConfOutputPluginKey: []interface{}{
						map[string]interface{}{
							agent.RegionKey:        "us-west-2",
							csm.MemoryLimitInMbKey: csm.DefaultMemoryLimitInMb,
							csm.LogLevelKey:        1,
						},
					},
				},
				"inputs": map[string]interface{}{
					ConfInputPluginKey: []interface{}{
						map[string]interface{}{
							ConfInputAddressKey: []string{
								computeIPv4LoopbackAddressFromPort(csm.DefaultPort),
								computeIPv6LoopbackAddressFromPort(csm.DefaultPort),
							},
							csm.DataFormatKey: "aws_csm",
						},
					},
				},
			},
		},
		"port and memory": {
			Input: `{"csm":{"port":2000,"memory_limit_in_mb":10}}`,
			Expect: map[string]interface{}{
				"outputs": map[string]interface{}{
					ConfOutputPluginKey: []interface{}{
						map[string]interface{}{
							agent.RegionKey:        "us-west-2",
							csm.MemoryLimitInMbKey: overrideMemoryLimit,
							csm.LogLevelKey:        csm.DefaultLogLevel,
						},
					},
				},
				"inputs": map[string]interface{}{
					ConfInputPluginKey: []interface{}{
						map[string]interface{}{
							ConfInputAddressKey: []string{
								computeIPv4LoopbackAddressFromPort(overridePort),
								computeIPv6LoopbackAddressFromPort(overridePort),
							},
							csm.DataFormatKey: "aws_csm",
						},
					},
				},
			},
		},
		"endpoint override": {
			Input: `{"csm":{"endpoint_override":"https://example.com"}}`,
			Expect: map[string]interface{}{
				"outputs": map[string]interface{}{
					ConfOutputPluginKey: []interface{}{
						map[string]interface{}{
							agent.RegionKey:         "us-west-2",
							csm.MemoryLimitInMbKey:  csm.DefaultMemoryLimitInMb,
							csm.EndpointOverrideKey: "https://example.com",
							csm.LogLevelKey:         csm.DefaultLogLevel,
						},
					},
				},
				"inputs": map[string]interface{}{
					ConfInputPluginKey: []interface{}{
						map[string]interface{}{
							ConfInputAddressKey: []string{
								computeIPv4LoopbackAddressFromPort(csm.DefaultPort),
								computeIPv6LoopbackAddressFromPort(csm.DefaultPort),
							},
							csm.DataFormatKey: "aws_csm",
						},
					},
				},
			},
		},
		"service_addresses": {
			Input: `{"csm":{"service_addresses":["udp://127.0.0.1:38000"]}}`,
			Expect: map[string]interface{}{
				"outputs": map[string]interface{}{
					ConfOutputPluginKey: []interface{}{
						map[string]interface{}{
							agent.RegionKey:        "us-west-2",
							csm.MemoryLimitInMbKey: csm.DefaultMemoryLimitInMb,
							csm.LogLevelKey:        csm.DefaultLogLevel,
						},
					},
				},
				"inputs": map[string]interface{}{
					ConfInputPluginKey: []interface{}{
						map[string]interface{}{
							ConfInputAddressKey: []string{
								"udp://127.0.0.1:38000",
							},
							csm.DataFormatKey: "aws_csm",
						},
					},
				},
			},
		},
	}

	for name, c := range cases {
		csmVal := new(Csm)
		t.Run(name, func(t *testing.T) {
			var input interface{}
			err := json.Unmarshal([]byte(c.Input), &input)
			assert.NoError(t, err)

			_, actual := csmVal.ApplyRule(input)
			assert.Equal(t, c.Expect, actual)
		})
	}
}
