// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers_test

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm"
	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/providers"
)

func TestMetricType(t *testing.T) {
	cases := []struct {
		name                  string
		MetricType            *string
		expectedIsNone        bool
		expectedIsFrequency   bool
		expectedIsSEH         bool
		expectedError         bool
		expectedContinueError bool
	}{
		{
			name:                  "nil case",
			expectedError:         true,
			expectedContinueError: true,
		},
		{
			name:          "empty case",
			MetricType:    aws.String(""),
			expectedError: true,
		},
		{
			name:          "invalid enum case",
			MetricType:    aws.String("invalid_enum"),
			expectedError: true,
		},
		{
			name:           "none case",
			MetricType:     aws.String(csm.MonitoringEventEntryMetricTypeNone),
			expectedIsNone: true,
		},
		{
			name:                "frequency case",
			MetricType:          aws.String(csm.MonitoringEventEntryMetricTypeFrequency),
			expectedIsFrequency: true,
		},
		{
			name:          "SEH case",
			MetricType:    aws.String(csm.MonitoringEventEntryMetricTypeSeh),
			expectedIsSEH: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := providers.NewMetricType(c.MetricType)

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

			if e, a := c.expectedIsFrequency, m.IsFrequency(); e != a {
				t.Errorf("expected %t, but received %t", e, a)
			}

			if e, a := c.expectedIsSEH, m.IsSEH(); e != a {
				t.Errorf("expected %t, but received %t", e, a)
			}
		})
	}
}
