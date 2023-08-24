// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"os"
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
			c := NewMetadataProvider(tc.sess, 0)
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

	// For build environments where IMDS is disabled via environment variable, explicitly re-enable it.  Otherwise the
	// call to c.InstanceId() fails before even contacting the mock session.
	// See https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html#envvars-list-AWS_EC2_METADATA_DISABLED
	const awsEc2MetadataDisabledEnvVar = "AWS_EC2_METADATA_DISABLED"
	val := os.Getenv(awsEc2MetadataDisabledEnvVar)
	defer func() { assert.NoError(t, os.Setenv(awsEc2MetadataDisabledEnvVar, val)) }()
	assert.NoError(t, os.Setenv(awsEc2MetadataDisabledEnvVar, "false"))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewMetadataProvider(tc.sess, 0)
			_, err := c.InstanceID(tc.ctx)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
