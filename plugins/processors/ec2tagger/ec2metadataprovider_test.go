// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/stretchr/testify/assert"
)

func TestMetadataProvider_Get(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		sess      *session.Session
		expectDoc ec2metadata.EC2InstanceIdentityDocument
	}{
		{
			name:      "mock session",
			ctx:       context.Background(),
			sess:      mock.Session,
			expectDoc: ec2metadata.EC2InstanceIdentityDocument{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewMetadataProvider(tc.sess)
			gotDoc, err := c.Get(tc.ctx)
			assert.NotNil(t, err)
			assert.Truef(t, reflect.DeepEqual(gotDoc, tc.expectDoc), "get() gotDoc: %v, expected: %v", gotDoc, tc.expectDoc)
		})
	}
}

func TestMetadataProvider_available(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		sess *session.Session
		want error
	}{
		{
			name: "mock session",
			ctx:  context.Background(),
			sess: mock.Session,
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewMetadataProvider(tc.sess)
			_, err := c.InstanceID(tc.ctx)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
