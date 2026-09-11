// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
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
