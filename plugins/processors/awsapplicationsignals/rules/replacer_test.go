// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type TestCaseForReplacer struct {
	name    string
	input   pcommon.Map
	output  pcommon.Map
	isTrace bool
}

func TestReplacerProcess(t *testing.T) {

	config := []Rule{
		{
			Selectors: []Selector{
				{
					Dimension: "Operation",
					Match:     "PUT *",
				},
				{
					Dimension: "RemoteService",
					Match:     "customer-test",
				},
			},
			Action: "keep",
		},
		{
			Selectors: []Selector{
				{
					Dimension: "RemoteService",
					Match:     "UnknownRemoteService",
				},
				{
					Dimension: "RemoteOperation",
					Match:     "GetShardIterator",
				},
			},
			Action: "drop",
		},
		{
			Selectors: []Selector{
				{
					Dimension: "Operation",
					Match:     "* /api/visits/*",
				},
				{
					Dimension: "RemoteOperation",
					Match:     "*",
				},
			},
			Replacements: []Replacement{
				{
					TargetDimension: "RemoteOperation",
					Value:           "ListPetsByCustomer",
				},
				{
					TargetDimension: "Operation",
					Value:           "PUT/GET",
				},
			},
			Action: "replace",
		},
	}

	testReplacer := NewReplacer(config, false)
	assert.Equal(t, 1, len(testReplacer.Actions))

	testCases := []TestCaseForReplacer{
		{
			name: "test01TraceMatch",
			input: generateTestAttributes("replace-test", "PUT /api/visits/test/123456", "customer-test",
				"GET", true),
			output: generateTestAttributes("replace-test", "PUT/GET", "customer-test",
				"ListPetsByCustomer", true),
			isTrace: true,
		},
		{
			name: "test02TraceNotMatch",
			input: generateTestAttributes("replace-test", "PUT /api/customer/owners/12345", "customer-test",
				"GET", true),
			output: generateTestAttributes("replace-test", "PUT /api/customer/owners/12345", "customer-test",
				"GET", true),
			isTrace: true,
		},
		{
			name: "test03MetricMatch",
			input: generateTestAttributes("replace-test", "PUT /api/visits/owners/12345", "customer-test",
				"GET", false),
			output: generateTestAttributes("replace-test", "PUT/GET", "customer-test",
				"ListPetsByCustomer", false),
			isTrace: false,
		},
		{
			name: "test04MetricNotMatch",
			input: generateTestAttributes("replace-test", "PUT /api/customer/owners/12345", "customer-test",
				"GET", false),
			output: generateTestAttributes("replace-test", "PUT /api/customer/owners/12345", "customer-test",
				"GET", false),
			isTrace: false,
		},
	}

	testMapPlaceHolder := pcommon.NewMap()
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, testReplacer.Process(tt.input, testMapPlaceHolder, tt.isTrace))
			assert.Equal(t, tt.output, tt.input)
		})
	}
}

func TestAddManagedDimensionKey(t *testing.T) {
	config := []Rule{
		{
			Selectors: []Selector{
				{
					Dimension: "Service",
					Match:     "app",
				},
				{
					Dimension: "RemoteService",
					Match:     "remote-app",
				},
			},
			Replacements: []Replacement{
				{
					TargetDimension: "RemoteEnvironment",
					Value:           "test",
				},
			},
			Action: "replace",
		},
	}

	testReplacer := NewReplacer(config, false)
	assert.Equal(t, 1, len(testReplacer.Actions))

	testCases := []TestCaseForReplacer{
		{
			name: "testAddMissingRemoteEnvironmentInMetric",
			input: generateAttributesWithEnv("app", "PUT /api/customer/owners/12345", "test",
				"remote-app", "GET", "", false),
			output: generateAttributesWithEnv("app", "PUT /api/customer/owners/12345", "test",
				"remote-app", "GET", "test", false),
			isTrace: false,
		},
		{
			name: "testAddMissingRemoteEnvironmentInTrace",
			input: generateAttributesWithEnv("app", "PUT /api/customer/owners/12345", "test",
				"remote-app", "GET", "", true),
			output: generateAttributesWithEnv("app", "PUT /api/customer/owners/12345", "test",
				"remote-app", "GET", "test", true),
			isTrace: true,
		},
		{
			name: "testReplaceRemoteEnvironmentInMetric",
			input: generateAttributesWithEnv("app", "PUT /api/customer/owners/12345", "test",
				"remote-app", "GET", "error", false),
			output: generateAttributesWithEnv("app", "PUT /api/customer/owners/12345", "test",
				"remote-app", "GET", "test", false),
			isTrace: false,
		},
		{
			name: "testReplaceRemoteEnvironmentInTrace",
			input: generateAttributesWithEnv("app", "PUT /api/customer/owners/12345", "test",
				"remote-app", "GET", "error", true),
			output: generateAttributesWithEnv("app", "PUT /api/customer/owners/12345", "test",
				"remote-app", "GET", "test", true),
			isTrace: true,
		},
	}

	testMapPlaceHolder := pcommon.NewMap()
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, testReplacer.Process(tt.input, testMapPlaceHolder, tt.isTrace))
			assert.Equal(t, tt.output, tt.input)
		})
	}
}

func TestReplacerProcessWithPriority(t *testing.T) {

	config := []Rule{
		{
			Selectors: []Selector{
				{
					Dimension: "Operation",
					Match:     "* /api/visits/*",
				},
				{
					Dimension: "RemoteOperation",
					Match:     "*",
				},
			},
			Replacements: []Replacement{
				{
					TargetDimension: "RemoteOperation",
					Value:           "ListPetsByCustomer",
				},
				{
					TargetDimension: "Operation",
					Value:           "PUT/GET",
				},
			},
			Action: "replace",
		},
		{
			Selectors: []Selector{
				{
					Dimension: "Operation",
					Match:     "PUT /api/visits/*",
				},
				{
					Dimension: "RemoteOperation",
					Match:     "PUT *",
				},
			},
			Replacements: []Replacement{
				{
					TargetDimension: "RemoteOperation",
					Value:           "PUT visits",
				},
				{
					TargetDimension: "Operation",
					Value:           "PUT",
				},
			},
			Action: "replace",
		},
	}

	testReplacer := NewReplacer(config, false)
	testMapPlaceHolder := pcommon.NewMap()

	testCases := []TestCaseForReplacer{
		{
			name: "test01TraceMatchPreviousOne",
			input: generateTestAttributes("replace-test", "PUT /api/visits/test/123456", "customer-test",
				"GET", true),
			output: generateTestAttributes("replace-test", "PUT/GET", "customer-test",
				"ListPetsByCustomer", true),
			isTrace: true,
		},
		{
			name: "test02TraceBothMatch",
			input: generateTestAttributes("replace-test", "PUT /api/visits/test/123456", "customer-test",
				"PUT /api/owners/123456", true),
			output: generateTestAttributes("replace-test", "PUT", "customer-test",
				"PUT visits", true),
			isTrace: true,
		},
		{
			name: "test03MetricMatchPreviousOne",
			input: generateTestAttributes("replace-test", "PUT /api/visits/owners/12345", "customer-test",
				"GET", false),
			output: generateTestAttributes("replace-test", "PUT/GET", "customer-test",
				"ListPetsByCustomer", false),
			isTrace: false,
		},
		{
			name: "test04MetricBothMatch",
			input: generateTestAttributes("replace-test", "PUT /api/visits/owners/12345", "customer-test",
				"PUT owners", false),
			output: generateTestAttributes("replace-test", "PUT", "customer-test",
				"PUT visits", false),
			isTrace: false,
		},
	}
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, testReplacer.Process(tt.input, testMapPlaceHolder, tt.isTrace))
			assert.Equal(t, tt.output, tt.input)
		})
	}
}

func TestReplacerProcessWithNilConfig(t *testing.T) {

	testReplacer := NewReplacer(nil, false)
	testMapPlaceHolder := pcommon.NewMap()

	testCases := []TestCaseForReplacer{
		{
			name: "test01Trace",
			input: generateTestAttributes("replace-test", "PUT /api/visits/test/123456", "customer-test",
				"GET", true),
			output: generateTestAttributes("replace-test", "PUT /api/visits/test/123456", "customer-test",
				"GET", true),
			isTrace: true,
		},
		{
			name: "test02Metric",
			input: generateTestAttributes("replace-test", "PUT /api/visits/owners/12345", "customer-test",
				"GET", false),
			output: generateTestAttributes("replace-test", "PUT /api/visits/owners/12345", "customer-test",
				"GET", false),
			isTrace: false,
		},
	}
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, testReplacer.Process(tt.input, testMapPlaceHolder, tt.isTrace))
			assert.Equal(t, tt.output, tt.input)
		})
	}
}

func TestReplacerProcessWithEmptyConfig(t *testing.T) {

	config := []Rule{}

	testReplacer := NewReplacer(config, false)
	testMapPlaceHolder := pcommon.NewMap()

	testCases := []TestCaseForReplacer{
		{
			name: "test01Trace",
			input: generateTestAttributes("replace-test", "PUT /api/visits/test/123456", "customer-test",
				"GET", true),
			output: generateTestAttributes("replace-test", "PUT /api/visits/test/123456", "customer-test",
				"GET", true),
			isTrace: true,
		},
		{
			name: "test02Metric",
			input: generateTestAttributes("replace-test", "PUT /api/visits/owners/12345", "customer-test",
				"GET", false),
			output: generateTestAttributes("replace-test", "PUT /api/visits/owners/12345", "customer-test",
				"GET", false),
			isTrace: false,
		},
	}
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, testReplacer.Process(tt.input, testMapPlaceHolder, tt.isTrace))
			assert.Equal(t, tt.output, tt.input)
		})
	}
}
