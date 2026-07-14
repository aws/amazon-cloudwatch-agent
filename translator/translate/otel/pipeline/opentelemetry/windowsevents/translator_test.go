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
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{name: "system_0"}}
	assert.Equal(t, pipeline.NewIDWithName(pipeline.SignalLogs, "windows_events_system_0"), pt.ID())
}

func TestPipelineTranslator_Translate_NoFilter(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:         "system",
		receiverName: "system",
		channel:      "System",
		raw:          false,
		resource:     map[string]string{"aws.log.source": "windows_events"},
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

func TestPipelineTranslator_Translate_WithFilter(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:         "system",
		receiverName: "system",
		channel:      "System",
		raw:          false,
		resource:     map[string]string{"aws.log.source": "windows_events"},
		eventLevels:  []string{"ERROR", "WARNING"},
	}}
	result, err := pt.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.Extensions.Len())
	assert.Equal(t, 2, result.Processors.Len())
}

func TestPipelineTranslator_Translate_WithRoutingAttrs(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:         "system",
		receiverName: "system",
		channel:      "System",
		resource:     map[string]string{"aws.log.source": "windows_events", "aws.log.channel": "System"},
		logGroupName: "/custom/group",
	}}
	result, err := pt.Translate(nil)
	require.NoError(t, err)

	// resource processor + scope transform = 2 processors
	assert.Equal(t, 2, result.Processors.Len())
}

func TestPipelineTranslator_DuplicateChannels_SharedReceiver(t *testing.T) {
	pt1 := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:         "system",
		receiverName: "system",
		channel:      "System",
		resource:     map[string]string{"aws.log.source": "windows_events", "aws.log.channel": "System"},
		eventLevels:  []string{"ERROR"},
	}}
	pt2 := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:         "system_1",
		receiverName: "system",
		channel:      "System",
		resource:     map[string]string{"aws.log.source": "windows_events", "aws.log.channel": "System"},
		eventLevels:  []string{"WARNING"},
	}}

	r1, err := pt1.Translate(nil)
	require.NoError(t, err)
	r2, err := pt2.Translate(nil)
	require.NoError(t, err)

	// Different pipeline IDs
	assert.NotEqual(t, pt1.ID(), pt2.ID())

	// Same receiver ID (shared checkpoint)
	assert.Equal(t, r1.Receivers.Keys(), r2.Receivers.Keys())
}

func TestPipelineTranslator_Translate_XmlWithEventIDs_Error(t *testing.T) {
	pt := &windowsEventsPipelineTranslator{entry: eventEntry{
		name:     "security",
		channel:  "Security",
		raw:      true,
		eventIDs: []int{4624},
	}}
	_, err := pt.Translate(nil)
	assert.EqualError(t, err, `event_ids filtering is not supported with event_format "xml" for channel "Security"`)
}

func TestBuildFilterCondition(t *testing.T) {
	tests := []struct {
		name     string
		entry    eventEntry
		expected string
	}{
		{
			name:     "no filter",
			entry:    eventEntry{name: "system_0", channel: "System"},
			expected: "",
		},
		{
			name:     "levels only",
			entry:    eventEntry{name: "system_0", eventLevels: []string{"ERROR", "WARNING"}},
			expected: `not((severity_number == 17 or severity_number == 13))`,
		},
		{
			name:     "ids only",
			entry:    eventEntry{name: "security_0", eventIDs: []int{4624, 4625}},
			expected: `not((body["event_id"]["id"] == 4624 or body["event_id"]["id"] == 4625))`,
		},
		{
			name:     "verbose only",
			entry:    eventEntry{name: "system_0", eventLevels: []string{"VERBOSE"}},
			expected: `not((severity_number == 0))`,
		},
		{
			name:     "levels and ids",
			entry:    eventEntry{name: "system_0", eventLevels: []string{"ERROR"}, eventIDs: []int{1001}},
			expected: `not((severity_number == 17) and (body["event_id"]["id"] == 1001))`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.filterCondition())
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
			entry:    eventEntry{name: "system"},
			expected: map[string]string{},
		},
		{
			name:     "log group only",
			entry:    eventEntry{name: "system", logGroupName: "/custom/group"},
			expected: map[string]string{"aws.log.group.name": "/custom/group"},
		},
		{
			name:     "both",
			entry:    eventEntry{name: "system", logGroupName: "/custom/group", logStreamName: "my-stream"},
			expected: map[string]string{"aws.log.group.name": "/custom/group", "aws.log.stream.name": "my-stream"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.routingAttributes())
		})
	}
}
