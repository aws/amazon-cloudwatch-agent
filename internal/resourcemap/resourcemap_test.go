// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcemap

import (
	"reflect"
	"testing"
)

func resetResourceMap() {
	resourceMap = nil
}

func TestGetResourceMap(t *testing.T) {
	tests := []struct {
		name string
		want *ResourceMap
	}{
		{
			name: "happypath",
			want: resourceMap,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetResourceMap(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetResourceMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitResourceMap(t *testing.T) {
	tests := []struct {
		name string
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitResourceMap()
		})
	}
}

func TestResourceMap_EC2Info(t *testing.T) {
	type fields struct {
		mode     string
		ec2Info  ec2Info
		ecsInfo  ecsInfo
		eksInfo  eksInfo
		logFiles map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   ec2Info
	}{
		{
			name: "happypath",
			fields: fields{
				ec2Info: ec2Info{InstanceID: "i-1234567890", AutoScalingGroup: "test-asg"},
			},
			want: ec2Info{
				InstanceID:       "i-1234567890",
				AutoScalingGroup: "test-asg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceMap{
				mode:     tt.fields.mode,
				ec2Info:  tt.fields.ec2Info,
				ecsInfo:  tt.fields.ecsInfo,
				eksInfo:  tt.fields.eksInfo,
				logFiles: tt.fields.logFiles,
			}
			if got := r.EC2Info(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EC2Info() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceMap_ECSInfo(t *testing.T) {
	type fields struct {
		mode     string
		ec2Info  ec2Info
		ecsInfo  ecsInfo
		eksInfo  eksInfo
		logFiles map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   ecsInfo
	}{
		{
			name: "happypath",
			fields: fields{
				ecsInfo: ecsInfo{ClusterName: "test-cluster"},
			},
			want: ecsInfo{
				ClusterName: "test-cluster",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceMap{
				mode:     tt.fields.mode,
				ec2Info:  tt.fields.ec2Info,
				ecsInfo:  tt.fields.ecsInfo,
				eksInfo:  tt.fields.eksInfo,
				logFiles: tt.fields.logFiles,
			}
			if got := r.ECSInfo(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ECSInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceMap_EKSInfo(t *testing.T) {
	type fields struct {
		mode     string
		ec2Info  ec2Info
		ecsInfo  ecsInfo
		eksInfo  eksInfo
		logFiles map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   eksInfo
	}{
		{
			name: "happypath",
			fields: fields{
				eksInfo: eksInfo{ClusterName: "test-cluster"},
			},
			want: eksInfo{
				ClusterName: "test-cluster",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceMap{
				mode:     tt.fields.mode,
				ec2Info:  tt.fields.ec2Info,
				ecsInfo:  tt.fields.ecsInfo,
				eksInfo:  tt.fields.eksInfo,
				logFiles: tt.fields.logFiles,
			}
			if got := r.EKSInfo(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EKSInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceMap_LogFiles(t *testing.T) {
	type fields struct {
		mode     string
		ec2Info  ec2Info
		ecsInfo  ecsInfo
		eksInfo  eksInfo
		logFiles map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResourceMap{
				mode:     tt.fields.mode,
				ec2Info:  tt.fields.ec2Info,
				ecsInfo:  tt.fields.ecsInfo,
				eksInfo:  tt.fields.eksInfo,
				logFiles: tt.fields.logFiles,
			}
			if got := r.LogFiles(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LogFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}
