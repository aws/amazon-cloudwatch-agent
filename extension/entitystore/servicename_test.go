// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
)

type fakeMetadataProvider struct {
	tags     []string
	tagValue map[string]string
	tagsErr  error
	iamRole  string
	iamErr   error
}

func (p *fakeMetadataProvider) Get(context.Context) (imds.InstanceIdentityDocument, error) {
	return imds.InstanceIdentityDocument{}, nil
}
func (p *fakeMetadataProvider) Hostname(context.Context) (string, error)   { return "", nil }
func (p *fakeMetadataProvider) InstanceID(context.Context) (string, error) { return "", nil }
func (p *fakeMetadataProvider) InstanceTags(context.Context) ([]string, error) {
	return p.tags, p.tagsErr
}
func (p *fakeMetadataProvider) ClientIAMRole(context.Context) (string, error) {
	return p.iamRole, p.iamErr
}
func (p *fakeMetadataProvider) InstanceTagValue(_ context.Context, tagKey string) (string, error) {
	value, ok := p.tagValue[tagKey]
	if !ok {
		return "", errors.New("no such tag")
	}
	return value, nil
}

func TestServiceNameFromIMDS(t *testing.T) {
	testCases := map[string]struct {
		provider *fakeMetadataProvider
		want     string
	}{
		"ServiceTagWins": {
			provider: &fakeMetadataProvider{
				tags:     []string{"app", "Service", "Name"},
				tagValue: map[string]string{"Service": "svc-from-tag", "app": "app-from-tag"},
				iamRole:  "my-role",
			},
			want: "svc-from-tag",
		},
		"ApplicationBeforeApp": {
			provider: &fakeMetadataProvider{
				tags:     []string{"app", "APPLICATION"},
				tagValue: map[string]string{"APPLICATION": "application-value", "app": "app-value"},
			},
			want: "application-value",
		},
		"AppTag": {
			provider: &fakeMetadataProvider{
				tags:     []string{"Name", "app"},
				tagValue: map[string]string{"app": " app-value "},
			},
			want: "app-value",
		},
		"TagValueErrorFallsThroughToNextTag": {
			provider: &fakeMetadataProvider{
				tags:     []string{"service", "app"},
				tagValue: map[string]string{"app": "app-value"},
				iamRole:  "my-role",
			},
			want: "app-value",
		},
		"TagValueErrorFallsThroughToIamRole": {
			provider: &fakeMetadataProvider{
				tags:    []string{"service"},
				iamRole: "my-role",
			},
			want: "my-role",
		},
		"TagValueErrorAndNoIamRole": {
			provider: &fakeMetadataProvider{
				tags:   []string{"service"},
				iamErr: errors.New("no role"),
			},
			want: "",
		},
		"NoTagsIamRole": {
			provider: &fakeMetadataProvider{
				tagsErr: errors.New("tags not enabled"),
				iamRole: "my-role",
			},
			want: "my-role",
		},
		"NothingResolves": {
			provider: &fakeMetadataProvider{
				tagsErr: errors.New("tags not enabled"),
				iamErr:  errors.New("no role"),
			},
			want: "",
		},
		"BlankEverywhere": {
			provider: &fakeMetadataProvider{
				tags:     []string{"service"},
				tagValue: map[string]string{"service": "  "},
				iamRole:  " ",
			},
			want: "",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.want, ServiceNameFromIMDS(context.Background(), testCase.provider))
		})
	}
}
