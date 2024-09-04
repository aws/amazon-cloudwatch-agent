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
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

var mockedInstanceIdentityDoc = &ec2metadata.EC2InstanceIdentityDocument{
	InstanceID:   "i-01d2417c27a396e44",
	Region:       "us-east-1",
	InstanceType: "m5ad.large",
	ImageID:      "ami-09edd32d9b0990d49",
}

type mockEC2Client struct {
	ec2iface.EC2API
	withASG bool
}

// construct the return results for the mocked DescribeTags api
var (
	tagKey1 = "tagKey1"
	tagVal1 = "tagVal1"
	tagDes1 = ec2.TagDescription{Key: &tagKey1, Value: &tagVal1}
)

var (
	tagKey2 = "tagKey2"
	tagVal2 = "tagVal2"
	tagDes2 = ec2.TagDescription{Key: &tagKey2, Value: &tagVal2}
)

var (
	tagKey3 = "aws:autoscaling:groupName"
	tagVal3 = "ASG-1"
	tagDes3 = ec2.TagDescription{Key: &tagKey3, Value: &tagVal3}
)

func (m *mockEC2Client) DescribeTags(*ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error) {
	//all tags are returned when the ec2 metadata service knows about all tags
	var allTags ec2.DescribeTagsOutput
	if m.withASG {
		allTags = ec2.DescribeTagsOutput{
			NextToken: nil,
			Tags:      []*ec2.TagDescription{&tagDes1, &tagDes2, &tagDes3},
		}
	} else {
		allTags = ec2.DescribeTagsOutput{
			NextToken: nil,
			Tags:      []*ec2.TagDescription{&tagDes1, &tagDes2},
		}
	}

	return &allTags, nil
}

func mockEC2Provider(region string, credential *configaws.CredentialConfig) ec2iface.EC2API {
	return &mockEC2Client{withASG: true}
}

func TestSetInstanceIdAndRegion(t *testing.T) {
	type args struct {
		metadataProvider ec2metadataprovider.MetadataProvider
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    ec2Info
	}{
		{
			name: "happy path",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
			},
			wantErr: false,
			want: ec2Info{
				InstanceID: mockedInstanceIdentityDoc.InstanceID,
			},
		},
	}
	for _, tt := range tests {
		logger, _ := zap.NewDevelopment()
		t.Run(tt.name, func(t *testing.T) {
			ei := &ec2Info{
				metadataProvider: tt.args.metadataProvider,
				logger:           logger,
			}
			if err := ei.setInstanceId(); (err != nil) != tt.wantErr {
				t.Errorf("setInstanceId() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want.InstanceID, ei.InstanceID)
		})
	}
}

func TestRetrieveASGName(t *testing.T) {
	type args struct {
		ec2Client        ec2iface.EC2API
		metadataProvider ec2metadataprovider.MetadataProvider
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    ec2Info
	}{
		{
			name: "happy path",
			args: args{
				ec2Client:        &mockEC2Client{},
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: "aws:autoscaling:groupName", TagValue: tagVal3},
			},
			wantErr: false,
			want: ec2Info{
				AutoScalingGroup: tagVal3,
			},
		},
		{
			name: "happy path with multiple tags",
			args: args{
				ec2Client:        &mockEC2Client{},
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: "aws:autoscaling:groupName\nenv\nname", TagValue: tagVal3},
			},
			wantErr: false,
			want: ec2Info{
				AutoScalingGroup: tagVal3,
			},
		},
		{
			name: "Success IMDS tags call but no ASG",
			args: args{
				ec2Client:        &mockEC2Client{},
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: "name", TagValue: tagVal3},
			},
			wantErr: false,
			want: ec2Info{
				AutoScalingGroup: "",
			},
		},
	}
	for _, tt := range tests {
		logger, _ := zap.NewDevelopment()
		t.Run(tt.name, func(t *testing.T) {
			ei := &ec2Info{metadataProvider: tt.args.metadataProvider, logger: logger}
			if err := ei.retrieveAsgName(tt.args.ec2Client); (err != nil) != tt.wantErr {
				t.Errorf("retrieveAsgName() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want.AutoScalingGroup, ei.AutoScalingGroup)
		})
	}
}

func TestRetrieveASGNameWithDescribeTags(t *testing.T) {
	type args struct {
		ec2Client ec2iface.EC2API
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    ec2Info
	}{
		{
			name: "happy path",
			args: args{
				ec2Client: &mockEC2Client{withASG: true},
			},
			wantErr: false,
			want: ec2Info{
				AutoScalingGroup: tagVal3,
			},
		},
		{
			name: "Success Describe tags call but no ASG",
			args: args{
				ec2Client: &mockEC2Client{withASG: false},
			},
			wantErr: false,
			want: ec2Info{
				AutoScalingGroup: "",
			},
		},
	}
	for _, tt := range tests {
		logger, _ := zap.NewDevelopment()
		t.Run(tt.name, func(t *testing.T) {
			ei := &ec2Info{logger: logger}
			if err := ei.retrieveAsgNameWithDescribeTags(tt.args.ec2Client); (err != nil) != tt.wantErr {
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
		autoScalingGroup string
	}
	tests := []struct {
		name string
		args *ec2Info
		want want
	}{
		{
			name: "Happy path",
			args: &ec2Info{
				InstanceID:       "i-01d2417c27a396e44",
				AutoScalingGroup: "asg",
				logger:           logger,
			},
			want: want{
				instanceId:       "i-01d2417c27a396e44",
				autoScalingGroup: "asg",
			},
		},
		{
			name: "InstanceId too large",
			args: &ec2Info{
				InstanceID:       strings.Repeat("a", 20),
				AutoScalingGroup: "asg",
				logger:           logger,
			},
			want: want{
				instanceId:       "",
				autoScalingGroup: "asg",
			},
		},
		{
			name: "AutoScalingGroup too large",
			args: &ec2Info{
				InstanceID:       "i-01d2417c27a396e44",
				AutoScalingGroup: strings.Repeat("a", 256),
				logger:           logger,
			},
			want: want{
				instanceId:       "i-01d2417c27a396e44",
				autoScalingGroup: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.ignoreInvalidFields()
			assert.Equal(t, tt.want.instanceId, tt.args.InstanceID)
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
		want ec2Info
	}{
		{
			name: "AutoScalingGroupWithDescribeTags",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, InstanceTagError: true},
			},
			want: ec2Info{
				InstanceID: mockedInstanceIdentityDoc.InstanceID,
			},
		},
		{
			name: "AutoScalingGroupWithInstanceTags",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc, Tags: "aws:autoscaling:groupName", TagValue: tagVal3},
			},
			want: ec2Info{
				InstanceID: mockedInstanceIdentityDoc.InstanceID,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture the logger output
			var buf bytes.Buffer
			writer := zapcore.AddSync(&buf)

			// Create a custom zapcore.Core that writes to the buffer
			encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
			core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)

			logger := zap.New(core)
			done := make(chan struct{})

			ei := &ec2Info{
				metadataProvider: tt.args.metadataProvider,
				ec2Provider:      mockEC2Provider,
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
