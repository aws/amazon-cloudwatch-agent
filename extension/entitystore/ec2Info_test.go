// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"bytes"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

var mockedInstanceIdentityDoc = &ec2metadata.EC2InstanceIdentityDocument{
	InstanceID:   "i-01d2417c27a396e44",
	AccountID:    "874389809020",
	Region:       "us-east-1",
	InstanceType: "m5ad.large",
	ImageID:      "ami-09edd32d9b0990d49",
}

var mockedInstanceIdentityDocWithLargeInstanceId = &ec2metadata.EC2InstanceIdentityDocument{
	InstanceID:   "i-01d2417c27a396e44394824728",
	AccountID:    "874389809020",
	Region:       "us-east-1",
	InstanceType: "m5ad.large",
	ImageID:      "ami-09edd32d9b0990d49",
}

var (
	tagVal3 = "ASG-1"
)

func TestSetInstanceIDAccountID(t *testing.T) {
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
				InstanceID: mockedInstanceIdentityDoc.InstanceID,
				AccountID:  mockedInstanceIdentityDoc.AccountID,
			},
		},
		{
			name: "InstanceId too large",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDocWithLargeInstanceId},
			},
			wantErr: false,
			want: EC2Info{
				InstanceID: "",
				AccountID:  mockedInstanceIdentityDocWithLargeInstanceId.AccountID,
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
			if err := ei.setInstanceIDAccountID(); (err != nil) != tt.wantErr {
				t.Errorf("setInstanceIDAccountID() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want.InstanceID, ei.GetInstanceID())
			assert.Equal(t, tt.want.AccountID, ei.GetAccountID())
		})
	}
}

func TestLogMessageDoesNotIncludeResourceInfo(t *testing.T) {
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
			var buf bytes.Buffer

			logger := CreateTestLogger(&buf)
			done := make(chan struct{})

			ei := &EC2Info{
				metadataProvider: tt.args.metadataProvider,
				logger:           logger,
				done:             done,
			}
			go ei.initEc2Info()
			time.Sleep(3 * time.Second)

			logOutput := buf.String()
			log.Println(logOutput)
			assert.NotContains(t, logOutput, ei.GetInstanceID())
		})
	}
}

func TestNotInitIfMetadataProviderIsEmpty(t *testing.T) {
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
			var buf bytes.Buffer

			logger := CreateTestLogger(&buf)
			done := make(chan struct{})

			ei := &EC2Info{
				logger: logger,
				done:   done,
			}
			go ei.initEc2Info()
			time.Sleep(3 * time.Second)

			logOutput := buf.String()
			log.Println(logOutput)
			assert.NotContains(t, logOutput, "Initializing EC2Info")
			assert.NotContains(t, logOutput, "Finished initializing EC2Info")
		})
	}
}
