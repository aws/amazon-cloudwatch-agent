// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

import (
	"reflect"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm"
	"github.com/aws/aws-sdk-go/aws"
)

func TestAddSchema(t *testing.T) {
	cases := []struct {
		output      csm.GetPublishingSchemaOutput
		definitions EventEntryDefinitions
		expected    EventEntryDefinitions
	}{
		{
			definitions: EventEntryDefinitions{
				container: map[string]EventEntryDefinition{},
			},
			expected: EventEntryDefinitions{
				container: map[string]EventEntryDefinition{
					"foo_key_type": {
						KeyType: EventEntryKeyType(csm.MonitoringEventEntryKeyTypeAggregation),
						Type:    MetricType(csm.MonitoringEventEntryMetricTypeNone),
						Name:    "foo_key_type",
					},
					"frequency_metric": {
						KeyType: EventEntryKeyType(csm.MonitoringEventEntryKeyTypeNone),
						Type:    MetricType(csm.MonitoringEventEntryMetricTypeFrequency),
						Name:    "frequency_metric",
					},
					"seh_metric": {
						KeyType: EventEntryKeyType(csm.MonitoringEventEntryKeyTypeNone),
						Type:    MetricType(csm.MonitoringEventEntryMetricTypeSeh),
						Name:    "seh_metric",
					},
				},
			},
			output: csm.GetPublishingSchemaOutput{
				MonitoringEventEntrySchemas: []*csm.MonitoringEventEntrySchema{
					{
						Name:    aws.String("foo_key_type"),
						KeyType: aws.String(csm.MonitoringEventEntryKeyTypeAggregation),
					},
					{
						Name:       aws.String("frequency_metric"),
						MetricType: aws.String(csm.MonitoringEventEntryMetricTypeFrequency),
					},
					{
						Name:       aws.String("seh_metric"),
						MetricType: aws.String(csm.MonitoringEventEntryMetricTypeSeh),
					},
				},
			},
		},
	}

	for _, c := range cases {
		c.definitions.add(&c.output)

		if e, a := c.expected, c.definitions; !reflect.DeepEqual(e, a) {
			t.Errorf("expected %v, but received %v", e, a)
		}
	}
}
