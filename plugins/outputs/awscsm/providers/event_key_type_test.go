// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm/csm"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/awscsm/providers"
)

func TestEventEntryKeyType(t *testing.T) {
	cases := []struct {
		name                           string
		keyType                        *string
		expectedIsNone                 bool
		expectedIsAggregation          bool
		expectedIsAggregationTimestamp bool
		expectedIsSample               bool
		expectedContinueError          bool
		expectedError                  bool
	}{
		{
			name:                  "nil case",
			expectedError:         true,
			expectedContinueError: true,
		},
		{
			name:          "invalid enum",
			keyType:       aws.String("invalid_enum"),
			expectedError: true,
		},
		{
			name:          "empty case",
			keyType:       aws.String(""),
			expectedError: true,
		},
		{
			name:           "none case",
			keyType:        aws.String(csm.MonitoringEventEntryKeyTypeNone),
			expectedIsNone: true,
		},
		{
			name:                  "aggregation case",
			keyType:               aws.String(csm.MonitoringEventEntryKeyTypeAggregation),
			expectedIsAggregation: true,
		},
		{
			name:                           "aggregation timestamp case",
			keyType:                        aws.String(csm.MonitoringEventEntryKeyTypeAggregationTimestamp),
			expectedIsAggregationTimestamp: true,
		},
		{
			name:             "sample case",
			keyType:          aws.String(csm.MonitoringEventEntryKeyTypeSample),
			expectedIsSample: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := providers.NewEventEntryKeyType(c.keyType)

			if !c.expectedError && err != nil {
				t.Errorf("expected no error, but received %v", err)
			} else if c.expectedError && err == nil {
				t.Errorf("expected an error, but received none")
			} else if c.expectedError && err != nil {
				if v, ok := err.(providers.ContinueError); !ok && c.expectedContinueError {
					t.Errorf("expected continue error, but received something else, %v", err)
				} else if c.expectedContinueError && !v.Continue() {
					t.Errorf("expected continue to return true, but got false")
				}
			}

			if e, a := c.expectedIsNone, m.IsNone(); e != a {
				t.Errorf("expected %t, but received %t", e, a)
			}

			if e, a := c.expectedIsAggregation, m.IsAggregation(); e != a {
				t.Errorf("expected %t, but received %t", e, a)
			}

			if e, a := c.expectedIsAggregationTimestamp, m.IsAggregationTimestamp(); e != a {
				t.Errorf("expected %t, but received %t", e, a)
			}

			if e, a := c.expectedIsSample, m.IsSample(); e != a {
				t.Errorf("expected %t, but received %t", e, a)
			}
		})
	}
}
