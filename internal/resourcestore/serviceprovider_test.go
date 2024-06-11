// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcestore

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

func Test_serviceprovider_initServiceProvider(t *testing.T) {
	type args struct {
		metadataProvider ec2metadataprovider.MetadataProvider
	}
	tests := []struct {
		name    string
		args    args
		wantIAM string
	}{
		{
			name: "HappyPath_IAMRole",
			args: args{
				metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: nil},
			},
			wantIAM: "TestRole",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &serviceprovider{}
			s.startServiceProvider(tt.args.metadataProvider)
			assert.Equal(t, tt.wantIAM, s.iamRole)
		})
	}
}

func Test_serviceprovider_ServiceName(t *testing.T) {
	type fields struct {
		iamRole string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "HappyPath_IAMServiceName",
			fields: fields{
				iamRole: "MockIAM",
			},
			want: "MockIAM",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &serviceprovider{
				iamRole: tt.fields.iamRole,
			}
			assert.Equal(t, tt.want, s.ServiceName())
		})
	}
}

func Test_serviceprovider_getIAMRole(t *testing.T) {
	tests := []struct {
		name             string
		metadataProvider ec2metadataprovider.MetadataProvider
		want             string
	}{
		{
			name:             "Happypath_MockMetadata",
			metadataProvider: &mockMetadataProvider{InstanceIdentityDocument: nil},
			want:             "TestRole",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &serviceprovider{}
			s.getIAMRole(tt.metadataProvider)
			assert.Equal(t, tt.want, s.iamRole)
		})
	}
}
