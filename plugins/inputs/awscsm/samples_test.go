// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestSamples(t *testing.T) {
	// rand seed of 0 generates
	//
	// 0.9451961492941164
	// 0.24496508529377975
	// 0.6559562651954052
	// 0.05434383959970039
	// 0.36758720663245853
	// 0.2894804331565928
	// 0.19243860967493215
	// 0.6553321508148324
	// 0.897169713149801
	// 0.16735444255905835
	cases := []struct {
		threshold       float64
		raw             []map[string]interface{}
		expected        []bool
		expectedSamples []map[string]interface{}
	}{
		{
			threshold: 0.25,
			raw: []map[string]interface{}{
				{
					"0": 0,
				},
				{
					"1": 10,
				},
				{
					"2": 20,
				},
				{
					"3": 30,
				},
				{
					"4": 40,
				},
				{
					"5": 50,
				},
				{
					"6": 60,
				},
				{
					"7": 70,
				},
				{
					"8": 80,
				},
				{
					"9": 90,
				},
			},
			expected: []bool{
				false,
				true,
				false,
				true,
				false,
				false,
				true,
				false,
				false,
				true,
			},
			expectedSamples: []map[string]interface{}{
				{
					"1": 10,
				},
				{
					"3": 30,
				},
				{
					"6": 60,
				},
				{
					"9": 90,
				},
			},
		},
		{
			threshold: 0.5,
			raw: []map[string]interface{}{
				{
					"0": 0,
				},
				{
					"1": 10,
				},
				{
					"2": 20,
				},
				{
					"3": 30,
				},
				{
					"4": 40,
				},
				{
					"5": 50,
				},
				{
					"6": 60,
				},
				{
					"7": 70,
				},
				{
					"8": 80,
				},
				{
					"9": 90,
				},
			},
			expected: []bool{
				false,
				true,
				false,
				true,
				true,
				true,
				true,
				false,
				false,
				true,
			},
			expectedSamples: []map[string]interface{}{
				{
					"1": 10,
				},
				{
					"3": 30,
				},
				{
					"4": 40,
				},
				{
					"5": 50,
				},
				{
					"6": 60,
				},
				{
					"9": 90,
				},
			},
		},
	}

	for _, c := range cases {
		rand.Seed(0)
		s := newSamples()
		for i := 0; i < len(c.expected); i++ {
			a := s.ShouldAdd(c.threshold)
			if e := c.expected[i]; e != a {
				t.Errorf("expected %t, but received %t", e, a)
			}

			if a {
				s.Add(c.raw[i])
			}
		}

		if e, a := c.expectedSamples, s.list; !reflect.DeepEqual(e, a) {
			t.Errorf("expected %v, but received %v", e, a)
		}
	}
}
