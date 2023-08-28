// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

func Test_getCollectorParams(t *testing.T) {
	type args struct {
		factories otelcol.Factories
		provider  otelcol.ConfigProvider
	}
	tests := []struct {
		name string
		args args
		want otelcol.CollectorSettings
	}{
		{
			name: "BuildInfoIsSet",
			args: args{
				factories: otelcol.Factories{},
				provider:  nil,
			},
			want: otelcol.CollectorSettings{
				Factories:      otelcol.Factories{},
				ConfigProvider: nil,
				BuildInfo: component.BuildInfo{
					Command:     "CWAgent",
					Description: "CloudWatch Agent",
					Version:     "Unknown",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCollectorParams(tt.args.factories, tt.args.provider); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCollectorParams() = %v, want %v", got, tt.want)
			}
		})
	}
}
