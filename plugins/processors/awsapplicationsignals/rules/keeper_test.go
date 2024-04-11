// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type TestCaseForKeeper struct {
	name   string
	input  pcommon.Map
	output bool
}

func TestKeeperProcessor(t *testing.T) {
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
					TargetDimension: "ResourceTarget",
					Value:           " ",
				},
			},
			Action: "replace",
		},
	}

	testKeeper := NewKeeper(config, false)
	assert.Equal(t, 1, len(testKeeper.Actions))

	isTrace := false

	testCases := []TestCaseForKeeper{
		{
			name:   "commonTest01ShouldBeKept",
			input:  generateTestAttributes("visit-test", "PUT owners", "customer-test", "PUT owners", isTrace),
			output: false,
		},
		{
			name:   "commonTest02ShouldBeDropped",
			input:  generateTestAttributes("visit-test", "PUT owners", "vet-test", "PUT owners", isTrace),
			output: true,
		},
		{
			name:   "commonTest03ShouldBeDropped",
			input:  generateTestAttributes("vet-test", "GET owners", "customer-test", "PUT owners", isTrace),
			output: true,
		},
	}
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			result, err := testKeeper.ShouldBeDropped(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.output, result)
		})
	}
}

func TestKeeperProcessorWithNilConfig(t *testing.T) {
	testKeeper := NewKeeper(nil, false)
	isTrace := false

	testCases := []TestCaseForKeeper{
		{
			name:   "nilTest01ShouldBeKept",
			input:  generateTestAttributes("visit-test", "PUT owners", "customer-test", "PUT owners", isTrace),
			output: false,
		},
		{
			name:   "nilTest02ShouldBeKept",
			input:  generateTestAttributes("visit-test", "PUT owners", "vet-test", "PUT owners", isTrace),
			output: false,
		},
		{
			name:   "nilTest03ShouldBeKept",
			input:  generateTestAttributes("vet-test", "PUT owners", "visit-test", "PUT owners", isTrace),
			output: false,
		},
		{
			name:   "nilTest04ShouldBeKept",
			input:  generateTestAttributes("customer-test", "PUT owners", "visit-test", "PUT owners", isTrace),
			output: false,
		},
	}
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			result, err := testKeeper.ShouldBeDropped(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.output, result)
		})
	}
}

func TestKeeperProcessorWithEmptyConfig(t *testing.T) {

	config := []Rule{}

	testKeeper := NewKeeper(config, false)
	isTrace := false

	testCases := []TestCaseForKeeper{
		{
			name:   "emptyTest01ShouldBeKept",
			input:  generateTestAttributes("visit-test", "PUT owners", "customer-test", "PUT owners", isTrace),
			output: false,
		},
		{
			name:   "emptyTest02ShouldBeKept",
			input:  generateTestAttributes("visit-test", "PUT owners", "vet-test", "PUT owners", isTrace),
			output: false,
		},
		{
			name:   "emptyTest03ShouldBeKept",
			input:  generateTestAttributes("vet-test", "PUT owners", "visit-test", "PUT owners", isTrace),
			output: false,
		},
		{
			name:   "emptyTest04ShouldBeKept",
			input:  generateTestAttributes("customer-test", "PUT owners", "visit-test", "PUT owners", isTrace),
			output: false,
		},
	}
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			result, err := testKeeper.ShouldBeDropped(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.output, result)
		})
	}
}
