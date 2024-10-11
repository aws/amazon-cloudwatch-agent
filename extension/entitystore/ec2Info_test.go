// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"bytes"
	"log"
	"strings"
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

var (
	tagVal3 = "ASG-1"
)

func TestSetInstanceIdAndRegion(t *testing.T) {
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
			assert.Equal(t, tt.want.InstanceID, ei.InstanceID)
			assert.Equal(t, tt.want.AccountID, ei.AccountID)
		})
	}
}

func TestRetrieveASGName(t *testing.T) {
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
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"aws:autoscaling:groupName": tagVal3}},
			},
			wantErr: false,
			want: EC2Info{
				AutoScalingGroup: tagVal3,
			},
		},
		{
			name: "happy path with multiple tags",
			args: args{
				metadataProvider: &mockMetadataProvider{
					InstanceIdentityDocument: mockedInstanceIdentityDoc,
					Tags: map[string]string{
						"aws:autoscaling:groupName": tagVal3,
						"env":                       "test-env",
						"name":                      "test-name",
					}},
			},

			wantErr: false,
			want: EC2Info{
				AutoScalingGroup: tagVal3,
			},
		},
		{
			name: "Success IMDS tags call but no ASG",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: map[string]string{"name": tagVal3}},
			},
			wantErr: false,
			want: EC2Info{
				AutoScalingGroup: "",
			},
		},
	}
	for _, tt := range tests {
		logger, _ := zap.NewDevelopment()
		t.Run(tt.name, func(t *testing.T) {
			ei := &EC2Info{metadataProvider: tt.args.metadataProvider, logger: logger}
			if err := ei.retrieveAsgName(); (err != nil) != tt.wantErr {
				t.Errorf("retrieveAsgName() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want.AutoScalingGroup, ei.AutoScalingGroup)
		})
	}
}

func TestIgnoreInvalidFields(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	type want struct {
		instanceId       string
		accountId        string
		autoScalingGroup string
	}
	tests := []struct {
		name string
		args *EC2Info
		want want
	}{
		{
			name: "Happy path",
			args: &EC2Info{
				InstanceID:       "i-01d2417c27a396e44",
				AccountID:        "0123456789012",
				AutoScalingGroup: "asg",
				logger:           logger,
			},
			want: want{
				instanceId:       "i-01d2417c27a396e44",
				accountId:        "0123456789012",
				autoScalingGroup: "asg",
			},
		},
		{
			name: "InstanceId too large",
			args: &EC2Info{
				InstanceID:       strings.Repeat("a", 20),
				AccountID:        "0123456789012",
				AutoScalingGroup: "asg",
				logger:           logger,
			},
			want: want{
				instanceId:       "",
				accountId:        "0123456789012",
				autoScalingGroup: "asg",
			},
		},
		{
			name: "AutoScalingGroup too large",
			args: &EC2Info{
				InstanceID:       "i-01d2417c27a396e44",
				AccountID:        "0123456789012",
				AutoScalingGroup: strings.Repeat("a", 256),
				logger:           logger,
			},
			want: want{
				instanceId:       "i-01d2417c27a396e44",
				accountId:        "0123456789012",
				autoScalingGroup: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.ignoreInvalidFields()
			assert.Equal(t, tt.want.instanceId, tt.args.InstanceID)
			assert.Equal(t, tt.want.accountId, tt.args.AccountID)
			assert.Equal(t, tt.want.autoScalingGroup, tt.args.AutoScalingGroup)
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
			assert.NotContains(t, logOutput, ei.InstanceID)
			assert.NotContains(t, logOutput, ei.AutoScalingGroup)
		})
	}
}
