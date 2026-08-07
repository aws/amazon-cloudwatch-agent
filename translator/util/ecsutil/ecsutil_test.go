// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRegion(t *testing.T) {
	cases := []struct {
		name       string
		taskARN    string
		wantRegion string
	}{
		{
			name:       "valid short-format ARN",
			taskARN:    "arn:aws:ecs:us-east-1:123456789012:task/abc123",
			wantRegion: "us-east-1",
		},
		{
			name:       "valid long-format ARN with cluster name",
			taskARN:    "arn:aws:ecs:eu-west-2:123456789012:task/my-cluster/abc123",
			wantRegion: "eu-west-2",
		},
		{
			name:       "exactly 4 segments - boundary passing case",
			taskARN:    "arn:aws:ecs:us-east-1",
			wantRegion: "us-east-1",
		},
		{
			name:       "exactly 3 segments - boundary failing case",
			taskARN:    "arn:aws:ecs",
			wantRegion: "",
		},
		{
			name:       "single token - too few segments",
			taskARN:    "not-an-arn",
			wantRegion: "",
		},
		{
			name:       "empty string",
			taskARN:    "",
			wantRegion: "",
		},
		{
			name:       "4 segments with empty region field - structurally valid but missing region",
			taskARN:    "arn:aws:ecs::123456789012:task/abc",
			wantRegion: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &ecsUtil{}
			em := &ecsMetadataResponse{TaskARN: tc.taskARN}
			require.NotPanics(t, func() { e.parseRegion(em) })
			assert.Equal(t, tc.wantRegion, e.Region)
		})
	}
}
