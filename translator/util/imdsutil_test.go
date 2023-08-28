// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"gotest.tools/v3/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestLoadImdsRetriesCommonConfig(t *testing.T) {
	tests := []struct {
		name            string
		imdsConfig      *commonconfig.IMDS
		expectedRetries string
	}{
		{
			name:            "expect empty for nil",
			expectedRetries: "",
		},
		{
			name:            "expect empty for empty",
			expectedRetries: "",
			imdsConfig:      &commonconfig.IMDS{},
		},
		{
			name:            "expect set in common config 5",
			expectedRetries: "5",
			imdsConfig: &commonconfig.IMDS{
				ImdsRetries: aws.Int(5),
			},
		},
		{
			name:            "expect empty for invalid",
			expectedRetries: "",
			imdsConfig: &commonconfig.IMDS{
				ImdsRetries: aws.Int(-1),
			},
		},
		{
			name:            "expect 0 set in common config 0",
			expectedRetries: "0",
			imdsConfig: &commonconfig.IMDS{
				ImdsRetries: aws.Int(0),
			},
		},
	}
	for _, tt := range tests {
		func() {
			defer os.Setenv(envconfig.IMDS_NUMBER_RETRY, "")
			t.Run(tt.name, func(t *testing.T) {
				LoadImdsRetries(tt.imdsConfig)
				assert.Equal(t, os.Getenv(envconfig.IMDS_NUMBER_RETRY), tt.expectedRetries)
			})
		}()
	}
}
