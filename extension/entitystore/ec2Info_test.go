// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

var mockedInstanceIdentityDoc = &ec2metadata.EC2InstanceIdentityDocument{
	InstanceID:       "i-01d2417c27a396e44",
	AccountID:        "874389809020",
	Region:           "us-east-1",
	InstanceType:     "m5ad.large",
	ImageID:          "ami-09edd32d9b0990d49",
	AvailabilityZone: "us-east-1a",
}

var mockedInstanceIdentityDocWithLargeInstanceId = &ec2metadata.EC2InstanceIdentityDocument{
	InstanceID:       "i-01d2417c27a396e44394824728",
	AccountID:        "874389809020",
	Region:           "us-east-1",
	InstanceType:     "m5ad.large",
	ImageID:          "ami-09edd32d9b0990d49",
	AvailabilityZone: "us-east-1a",
}

var (
	tagVal3 = "ASG-1"
)

func TestSetEC2Metadata(t *testing.T) {
	t.Parallel()
	type args struct {
		metadataProvider ec2metadataprovider.MetadataProvider
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    EC2Info
	}{
		{
			name: "happy path",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
			},
			wantErr: false,
			want: EC2Info{
				InstanceID:       mockedInstanceIdentityDoc.InstanceID,
				AccountID:        mockedInstanceIdentityDoc.AccountID,
				InstanceType:     mockedInstanceIdentityDoc.InstanceType,
				ImageID:          mockedInstanceIdentityDoc.ImageID,
				AvailabilityZone: mockedInstanceIdentityDoc.AvailabilityZone,
				Hostname:         "MockHostName",
			},
		},
		{
			name: "InstanceId too large",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDocWithLargeInstanceId},
			},
			wantErr: false,
			want: EC2Info{
				InstanceID:       "",
				AccountID:        mockedInstanceIdentityDocWithLargeInstanceId.AccountID,
				InstanceType:     mockedInstanceIdentityDocWithLargeInstanceId.InstanceType,
				ImageID:          mockedInstanceIdentityDocWithLargeInstanceId.ImageID,
				AvailabilityZone: mockedInstanceIdentityDocWithLargeInstanceId.AvailabilityZone,
				Hostname:         "MockHostName",
			},
		},
	}
	for _, tt := range tests {
		logger, _ := zap.NewDevelopment()
		t.Run(tt.name, func(t *testing.T) {
			ei := &EC2Info{
				metadataProvider: tt.args.metadataProvider,
				logger:           logger,
			}
			if err := ei.setEC2Metadata(); (err != nil) != tt.wantErr {
				t.Errorf("setEC2Metadata() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want.InstanceID, ei.GetInstanceID())
			assert.Equal(t, tt.want.AccountID, ei.GetAccountID())
			assert.Equal(t, tt.want.InstanceType, ei.GetInstanceType())
			assert.Equal(t, tt.want.ImageID, ei.GetImageID())
			assert.Equal(t, tt.want.AvailabilityZone, ei.GetAvailabilityZone())
			assert.Equal(t, tt.want.Hostname, ei.GetHostname())
		})
	}
}

func TestLogMessageDoesNotIncludeResourceInfo(t *testing.T) {
	t.Parallel()
	type args struct {
		metadataProvider ec2metadataprovider.MetadataProvider
	}
	tests := []struct {
		name string
		args args
		want EC2Info
	}{
		{
			name: "AutoScalingGroupWithInstanceTags",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"aws:autoscaling:groupName": tagVal3}},
			},
			want: EC2Info{
				InstanceID: mockedInstanceIdentityDoc.InstanceID,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture the logger output
			var buf syncBuffer

			logger := CreateTestLogger(&buf)
			done := make(chan struct{})

			ei := &EC2Info{
				metadataProvider: tt.args.metadataProvider,
				logger:           logger,
				done:             done,
			}
			go ei.initEc2Info()
			require.Eventually(t, func() bool {
				return ei.GetInstanceID() != ""
			}, 3*time.Second, 100*time.Millisecond)

			logOutput := buf.String()
			log.Println(logOutput)
			assert.NotContains(t, logOutput, ei.GetInstanceID())
		})
	}
}

func TestNotInitIfMetadataProviderIsEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
	}{
		{
			name: "AutoScalingGroupWithInstanceTags",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture the logger output
			var buf syncBuffer

			logger := CreateTestLogger(&buf)
			done := make(chan struct{})

			ei := &EC2Info{
				logger: logger,
				done:   done,
			}
			finished := make(chan struct{})
			go func() {
				ei.initEc2Info()
				close(finished)
			}()
			require.Eventually(t, func() bool {
				select {
				case <-finished:
					return true
				default:
					return false
				}
			}, 3*time.Second, 100*time.Millisecond)

			logOutput := buf.String()
			log.Println(logOutput)
			assert.NotContains(t, logOutput, "Initializing EC2Info")
			assert.NotContains(t, logOutput, "Finished initializing EC2Info")
		})
	}
}

func TestGettersReturnEmptyBeforeInit(t *testing.T) {
	ei := &EC2Info{}
	assert.Equal(t, "", ei.GetInstanceID())
	assert.Equal(t, "", ei.GetAccountID())
	assert.Equal(t, "", ei.GetInstanceType())
	assert.Equal(t, "", ei.GetImageID())
	assert.Equal(t, "", ei.GetAvailabilityZone())
	assert.Equal(t, "", ei.GetHostname())
}

func TestHostnameFailureProceedsWithoutIt(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ei := &EC2Info{
		metadataProvider: &mockMetadataProvider{
			InstanceIdentityDocument: mockedInstanceIdentityDoc,
			HostnameError:            true,
		},
		logger: logger,
	}
	err := ei.setEC2Metadata()
	assert.NoError(t, err, "should succeed even when Hostname() fails")
	// Hostname is empty but all other fields are populated
	assert.Equal(t, "", ei.GetHostname())
	assert.Equal(t, mockedInstanceIdentityDoc.InstanceID, ei.GetInstanceID())
	assert.Equal(t, mockedInstanceIdentityDoc.InstanceType, ei.GetInstanceType())
	assert.Equal(t, mockedInstanceIdentityDoc.ImageID, ei.GetImageID())
	assert.Equal(t, mockedInstanceIdentityDoc.AvailabilityZone, ei.GetAvailabilityZone())
}
