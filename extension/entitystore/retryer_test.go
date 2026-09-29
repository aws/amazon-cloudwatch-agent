// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
)

func TestRetryer_refreshLoop(t *testing.T) {
	type fields struct {
		metadataProvider ec2metadataprovider.MetadataProvider
		iamRole          string
		oneTime          bool
	}
	tests := []struct {
		name        string
		fields      fields
		wantIamRole string
	}{
		{
			name: "HappyPath_CorrectRefresh",
			fields: fields{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: &imds.InstanceIdentityDocument{
						InstanceID: "i-123456789"},
				},
				iamRole: "original-role",
			},
			wantIamRole: "TestRole",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			done := make(chan struct{})
			s := &serviceprovider{
				metadataProvider: tt.fields.metadataProvider,
				iamRole:          tt.fields.iamRole,
				done:             done,
			}
			unlimitedRetryer := NewRetryer(tt.fields.oneTime, true, defaultJitterMin, defaultJitterMax, ec2tagger.BackoffSleepArray, infRetry, s.done, logger)
			go unlimitedRetryer.refreshLoop(s.scrapeIAMRole)
			time.Sleep(time.Second)
			close(done)
			assert.Equal(t, tt.wantIamRole, s.GetIAMRole())
		})
	}
}

func TestRetryer_refreshLoop_maxRetryBoundsFailures(t *testing.T) {
	logger := zap.NewNop()
	done := make(chan struct{})
	defer close(done)
	calls := 0
	failing := func() error {
		calls++
		return errors.New("always fails")
	}

	r := NewRetryer(true, true, 0, 1, []time.Duration{0}, 3, done, logger)
	r.refreshLoop(failing)
	assert.Equal(t, 3, calls)
}

func Test_serviceprovider_retryerBounds(t *testing.T) {
	s := &serviceprovider{done: make(chan struct{}), logger: zap.NewNop()}

	iam := s.newIAMRoleRetryer()
	assert.Equal(t, infRetry, iam.maxRetry)
	assert.False(t, iam.oneTime)

	tags := s.newInstanceTagsRetryer()
	assert.Equal(t, maxRetry, tags.maxRetry)
	assert.True(t, tags.oneTime)
}
