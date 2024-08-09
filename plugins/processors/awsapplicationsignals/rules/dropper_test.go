// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type TestCaseForDropper struct {
	name   string
	input  pcommon.Map
	output bool
}

func TestDropperProcessor(t *testing.T) {
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
					Match:     "customer-*",
				},
				{
					Dimension: "RemoteOperation",
					Match:     "GET /Owners/*",
				},
			},
			Action: "drop",
		},
		{
			Selectors: []Selector{
				{
					Dimension: "Operation",
					Match:     "PUT /*/pet/*",
				},
				{
					Dimension: "RemoteService",
					Match:     "visit-*-service",
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
					TargetDimension: "ResourceTarget",
					Value:           " ",
				},
			},
			Action: "replace",
		},
	}

	testDropper := NewDropper(config)
	assert.Equal(t, 2, len(testDropper.Actions))

	testCases := []TestCaseForDropper{
		{
			name:   "commonTest01ShouldBeKept",
			input:  generateTestAttributes("customer-test", "GET /user/123", "visit-service", "GET /visit/12345", false),
			output: false,
		},
		{
			name:   "commonTest02ShouldBeDropped",
			input:  generateTestAttributes("common-test", "GET /user/123", "customer-service", "GET /Owners/12345", false),
			output: true,
		},
		{
			name:   "commonTest03ShouldBeDropped",
			input:  generateTestAttributes("common-test", "PUT /test/pet/123", "visit-test-service", "GET /visit/12345", false),
			output: true,
		},
	}

	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			result, err := testDropper.ShouldBeDropped(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.output, result)
		})
	}
}

func TestDropperProcessorWithNilConfig(t *testing.T) {
	testDropper := NewDropper(nil)
	isTrace := false

	testCases := []TestCaseForDropper{
		{
			name:   "nilTest01ShouldBeKept",
			input:  generateTestAttributes("customer-test", "GET /user/123", "visit-service", "GET /visit/12345", isTrace),
			output: false,
		},
		{
			name:   "nilTest02ShouldBeDropped",
			input:  generateTestAttributes("common-test", "GET /user/123", "customer-service", "GET /Owners/12345", isTrace),
			output: false,
		},
		{
			name:   "nilTest03ShouldBeDropped",
			input:  generateTestAttributes("common-test", "PUT /test/pet/123", "visit-test-service", "GET /visit/12345", isTrace),
			output: false,
		},
	}

	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			result, err := testDropper.ShouldBeDropped(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.output, result)
		})
	}
}

func TestDropperProcessorWithEmptyConfig(t *testing.T) {
	var config []Rule

	testDropper := NewDropper(config)
	isTrace := false

	testCases := []TestCaseForDropper{
		{
			name:   "emptyTest01ShouldBeKept",
			input:  generateTestAttributes("customer-test", "GET /user/123", "visit-service", "GET /visit/12345", isTrace),
			output: false,
		},
		{
			name:   "emptyTest02ShouldBeDropped",
			input:  generateTestAttributes("common-test", "GET /user/123", "customer-service", "GET /Owners/12345", isTrace),
			output: false,
		},
		{
			name:   "emptyTest03ShouldBeDropped",
			input:  generateTestAttributes("common-test", "PUT /test/pet/123", "visit-test-service", "GET /visit/12345", isTrace),
			output: false,
		},
	}

	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			result, err := testDropper.ShouldBeDropped(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.output, result)
		})
	}
}
