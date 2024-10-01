// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
)

var (
	FastBackOffArray = []time.Duration{0, 0, 0}
)

func TestRetryer_refreshLoop(t *testing.T) {
	type fields struct {
		metadataProvider  ec2metadataprovider.MetadataProvider
		ec2API            ec2iface.EC2API
		iamRole           string
		ec2TagServiceName string
		oneTime           bool
	}
	type expectedInfo struct {
		iamRole           string
		ec2TagServiceName string
	}
	tests := []struct {
		name         string
		fields       fields
		expectedInfo expectedInfo
	}{
		{
			name: "HappyPath_CorrectRefresh",
			fields: fields{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						InstanceID: "i-123456789"},
				},
				ec2API:            &mockServiceNameEC2Client{},
				iamRole:           "original-role",
				ec2TagServiceName: "original-tag-name",
			},
			expectedInfo: expectedInfo{
				iamRole:           "TestRole",
				ec2TagServiceName: "test-service",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			done := make(chan struct{})
			s := &serviceprovider{
				metadataProvider: tt.fields.metadataProvider,
				ec2API:           tt.fields.ec2API,
				ec2Provider: func(s string, config *configaws.CredentialConfig) ec2iface.EC2API {
					return tt.fields.ec2API
				},
				iamRole:           tt.fields.iamRole,
				ec2TagServiceName: tt.fields.ec2TagServiceName,
				done:              done,
			}
			limitedRetryer := NewRetryer(tt.fields.oneTime, false, describeTagsJitterMin, describeTagsJitterMax, ec2tagger.ThrottleBackOffArray, maxRetry, s.done, logger)
			unlimitedRetryer := NewRetryer(tt.fields.oneTime, true, defaultJitterMin, defaultJitterMax, ec2tagger.BackoffSleepArray, infRetry, s.done, logger)
			go limitedRetryer.refreshLoop(s.getEC2TagServiceName)
			go unlimitedRetryer.refreshLoop(s.getIAMRole)
			time.Sleep(time.Second)
			close(done)
			assert.Equal(t, tt.expectedInfo.iamRole, s.iamRole)
			assert.Equal(t, tt.expectedInfo.ec2TagServiceName, s.ec2TagServiceName)
		})
	}
}

func TestRetryer_refreshLoopRetry(t *testing.T) {
	type fields struct {
		metadataProvider ec2metadataprovider.MetadataProvider
		ec2API           ec2iface.EC2API
		oneTime          bool
	}
	tests := []struct {
		name          string
		fields        fields
		expectedRetry int
	}{
		{
			name: "ThrottleLimitError",
			fields: fields{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						InstanceID: "i-123456789"},
				},
				ec2API: &mockServiceNameEC2Client{
					throttleError: true,
				},
			},
			expectedRetry: 4,
		},
		{
			name: "AuthError",
			fields: fields{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: &ec2metadata.EC2InstanceIdentityDocument{
						InstanceID: "i-123456789"},
				},
				ec2API: &mockServiceNameEC2Client{
					authError: true,
				},
			},
			expectedRetry: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			done := make(chan struct{})
			s := &serviceprovider{
				metadataProvider: tt.fields.metadataProvider,
				ec2API:           tt.fields.ec2API,
				ec2Provider: func(s string, config *configaws.CredentialConfig) ec2iface.EC2API {
					return tt.fields.ec2API
				},
				done: done,
			}
			limitedRetryer := NewRetryer(tt.fields.oneTime, false, describeTagsJitterMin, describeTagsJitterMax, FastBackOffArray, maxRetry, s.done, logger)
			retry := limitedRetryer.refreshLoop(s.getEC2TagServiceName)
			time.Sleep(time.Second)
			close(done)
			assert.Equal(t, tt.expectedRetry, retry)
		})
	}
}
