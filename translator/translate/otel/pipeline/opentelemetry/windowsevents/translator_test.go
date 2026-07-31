// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowsevents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"
)

func TestPipelineTranslator_ID(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{index: 0, channel: "System"}}
	assert.Equal(t, pipeline.NewIDWithName(pipeline.SignalLogs, "windows_events_system_0"), pt.ID())
}

func TestPipelineTranslator_Translate_NoFilter(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		index:    0,
		channel:  "System",
		format:   "",
		resource: map[string]string{"aws.log.source": "windows_events"},
	}}
	result, err := pt.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.Receivers.Len())
	assert.Equal(t, 1, result.Processors.Len())
	assert.Equal(t, 1, result.Exporters.Len())
	assert.Equal(t, 1, result.Extensions.Len())
	assert.Equal(t, 1, result.Connectors.Len())
}

func TestPipelineTranslator_Translate_WithLevels(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		index:       0,
		channel:     "System",
		format:      "",
		resource:    map[string]string{"aws.log.source": "windows_events"},
		eventLevels: []string{"ERROR", "WARNING"},
	}}
	result, err := pt.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Only scope transform processor (filtering is done at the receiver via query XML)
	assert.Equal(t, 1, result.Processors.Len())
	assert.Equal(t, 1, result.Extensions.Len())
}

func TestPipelineTranslator_Translate_WithRoutingAttrs(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		index:        0,
		channel:      "System",
		resource:     map[string]string{"aws.log.source": "windows_events", "aws.log.channel": "System"},
		logGroupName: "/custom/group",
	}}
	result, err := pt.Translate(nil)
	require.NoError(t, err)

	// resource processor + scope transform = 2 processors
	assert.Equal(t, 2, result.Processors.Len())
}

func TestPipelineTranslator_DifferentLevels_DifferentReceivers(t *testing.T) {
	pt1 := &windowsEventsPipelineTranslator{entry: eventEntry{
		index:       0,
		channel:     "System",
		resource:    map[string]string{"aws.log.source": "windows_events", "aws.log.channel": "System"},
		eventLevels: []string{"ERROR"},
	}}
	pt2 := &windowsEventsPipelineTranslator{entry: eventEntry{
		index:       1,
		channel:     "System",
		resource:    map[string]string{"aws.log.source": "windows_events", "aws.log.channel": "System"},
		eventLevels: []string{"WARNING"},
	}}

	r1, err := pt1.Translate(nil)
	require.NoError(t, err)
	r2, err := pt2.Translate(nil)
	require.NoError(t, err)

	// Different pipeline IDs
	assert.NotEqual(t, pt1.ID(), pt2.ID())

	// Different receiver IDs (different query XMLs)
	assert.NotEqual(t, r1.Receivers.Keys(), r2.Receivers.Keys())
}

func TestQueryXML(t *testing.T) {
	tests := []struct {
		name     string
		entry    eventEntry
		expected string
	}{
		{
			name:     "time cutoff only",
			entry:    eventEntry{channel: "System"},
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "levels with time cutoff",
			entry:    eventEntry{channel: "System", eventLevels: []string{"ERROR", "WARNING"}},
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(Level='2' or Level='3') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "ids with time cutoff",
			entry:    eventEntry{channel: "Security", eventIDs: []int{4624, 4625}},
			expected: `<QueryList><Query Id="0"><Select Path="Security">*[System[(EventID='4624' or EventID='4625') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "information includes level 0 and 4",
			entry:    eventEntry{channel: "System", eventLevels: []string{"INFORMATION"}},
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(Level='4' or Level='0') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "levels and ids with time cutoff",
			entry:    eventEntry{channel: "System", eventLevels: []string{"ERROR"}, eventIDs: []int{1001}},
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(Level='2') and (EventID='1001') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "xml format with ids",
			entry:    eventEntry{channel: "System", format: "xml", eventIDs: []int{100}},
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(EventID='100') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "all levels",
			entry:    eventEntry{channel: "System", eventLevels: []string{"CRITICAL", "ERROR", "WARNING", "INFORMATION", "VERBOSE"}},
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(Level='1' or Level='2' or Level='3' or Level='4' or Level='0' or Level='5') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "unknown level ignored",
			entry:    eventEntry{channel: "System", eventLevels: []string{"INVALID", "ERROR"}},
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(Level='2') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.queryXML())
		})
	}
}

func TestRoutingAttributes(t *testing.T) {
	tests := []struct {
		name     string
		entry    eventEntry
		expected map[string]string
	}{
		{
			name:     "no routing attrs",
			entry:    eventEntry{},
			expected: nil,
		},
		{
			name:     "log group only",
			entry:    eventEntry{logGroupName: "/custom/group"},
			expected: map[string]string{"aws.log.group.name": "/custom/group"},
		},
		{
			name:     "both",
			entry:    eventEntry{logGroupName: "/custom/group", logStreamName: "my-stream"},
			expected: map[string]string{"aws.log.group.name": "/custom/group", "aws.log.stream.name": "my-stream"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.routingAttributes())
		})
	}
}
