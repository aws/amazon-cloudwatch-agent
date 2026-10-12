// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceNameFromInstanceTags(t *testing.T) {
	testCases := map[string]struct {
		tags   map[string]string
		want   string
		wantOK bool
	}{
		"ServiceTagWins": {
			tags:   map[string]string{"Service": "svc-from-tag", "app": "app-from-tag", "Name": "ignored"},
			want:   "svc-from-tag",
			wantOK: true,
		},
		"ApplicationBeforeApp": {
			tags:   map[string]string{"APPLICATION": "application-value", "app": "app-value"},
			want:   "application-value",
			wantOK: true,
		},
		"AppTag": {
			tags:   map[string]string{"Name": "ignored", "app": "app-value"},
			want:   "app-value",
			wantOK: true,
		},
		"BlankValueIsStillAHit_LegacyBehaviour": {
			tags:   map[string]string{"service": "", "application": "application-value"},
			want:   "",
			wantOK: true,
		},
		"NoPriorityTag": {
			tags:   map[string]string{"Name": "ignored"},
			want:   "",
			wantOK: false,
		},
		"NoTags": {
			tags:   map[string]string{},
			want:   "",
			wantOK: false,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			provider := &mockMetadataProvider{Tags: testCase.tags}
			tagKeys, err := provider.InstanceTags(context.Background())
			assert.NoError(t, err)
			got, ok := ServiceNameFromInstanceTags(context.Background(), provider, tagKeys)
			assert.Equal(t, testCase.wantOK, ok)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestServiceNameFromInstanceTags_TagValueErrorContinues(t *testing.T) {
	provider := &mockMetadataProvider{Tags: map[string]string{"application": "application-value"}}
	got, ok := ServiceNameFromInstanceTags(context.Background(), provider, []string{"service", "application"})
	assert.True(t, ok)
	assert.Equal(t, "application-value", got)
}

func TestServiceNameFromIMDS(t *testing.T) {
	testCases := map[string]struct {
		provider *mockMetadataProvider
		want     string
	}{
		"ServiceTagWins": {
			provider: &mockMetadataProvider{Tags: map[string]string{"Service": "svc-from-tag", "app": "app-from-tag"}},
			want:     "svc-from-tag",
		},
		"TrimsValue": {
			provider: &mockMetadataProvider{Tags: map[string]string{"app": " app-value "}},
			want:     "app-value",
		},
		"BlankTagFallsThroughToNextKey": {
			provider: &mockMetadataProvider{Tags: map[string]string{"service": "  ", "application": "application-value"}},
			want:     "application-value",
		},
		"NoPriorityTagFallsBackToIamRole": {
			provider: &mockMetadataProvider{Tags: map[string]string{"Name": "ignored"}},
			want:     "TestRole",
		},
		"TagsErrorFallsBackToIamRole": {
			provider: &mockMetadataProvider{InstanceTagError: true},
			want:     "TestRole",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.want, ServiceNameFromIMDS(context.Background(), testCase.provider))
		})
	}
}
