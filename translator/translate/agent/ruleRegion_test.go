// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

func TestRegionRule(t *testing.T) {
	origDetectRegion := util.DetectRegion
	t.Cleanup(func() {
		util.DetectRegion = origDetectRegion
		context.ResetContext()
		translator.ResetMessages()
	})

	testCases := map[string]struct {
		input              string
		mode               string
		detectedRegion     string
		detectedRegionType string
		wantRegion         string
		wantRegionType     string
		wantError          bool
	}{
		"WithRegionInConfig": {
			input:          `{"region": "us-east-1"}`,
			mode:           config.ModeEC2,
			wantRegion:     "us-east-1",
			wantRegionType: config.RegionTypeAgentConfigJson,
		},
		"WithDetectedRegion": {
			input:              `{}`,
			mode:               config.ModeEC2,
			detectedRegion:     "us-west-2",
			detectedRegionType: config.RegionTypeEC2Metadata,
			wantRegion:         "us-west-2",
			wantRegionType:     config.RegionTypeEC2Metadata,
		},
		"WithMissingRegion/EC2": {
			input:              `{}`,
			mode:               config.ModeEC2,
			detectedRegionType: config.RegionTypeNotFound,
			wantRegionType:     config.RegionTypeNotFound,
			wantError:          true,
		},
		"WithMissingRegion/AzureVM": {
			input:              `{}`,
			mode:               config.ModeAzureVM,
			detectedRegionType: config.RegionTypeNotFound,
			wantRegion:         "${AWS_REGION}",
			wantRegionType:     config.RegionTypeNotFound,
		},
		"WithDetectedRegion/AzureVM": {
			input:              `{}`,
			mode:               config.ModeAzureVM,
			detectedRegion:     "us-west-2",
			detectedRegionType: config.RegionTypeCredsMap,
			wantRegion:         "us-west-2",
			wantRegionType:     config.RegionTypeCredsMap,
		},
		"WithMissingRegion/GCE": {
			input:              `{}`,
			mode:               config.ModeGCE,
			detectedRegionType: config.RegionTypeNotFound,
			wantRegion:         "${AWS_REGION}",
			wantRegionType:     config.RegionTypeNotFound,
		},
		"WithDetectedRegion/GCE": {
			input:              `{}`,
			mode:               config.ModeGCE,
			detectedRegion:     "us-west-2",
			detectedRegionType: config.RegionTypeCredsMap,
			wantRegion:         "us-west-2",
			wantRegionType:     config.RegionTypeCredsMap,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			translator.ResetMessages()
			context.ResetContext()
			context.CurrentContext().SetMode(testCase.mode)
			util.DetectRegion = func(string, map[string]string) (string, string) {
				return testCase.detectedRegion, testCase.detectedRegionType
			}

			var input any
			require.NoError(t, json.Unmarshal([]byte(testCase.input), &input))

			r := new(Region)
			r.ApplyRule(input)

			assert.Equal(t, testCase.wantRegion, Global_Config.Region)
			assert.Equal(t, testCase.wantRegionType, Global_Config.RegionType)
			if testCase.wantError {
				assert.NotEmpty(t, translator.ErrorMessages)
			} else {
				assert.Empty(t, translator.ErrorMessages)
			}
		})
	}
}

// TestRegionRule_RealDetection runs the rule against the real util.DetectRegion (no stub) to pin
// the precedence agent.region > environment for a mode with no metadata fallback, and to show
// that an onPrem host without agent.region resolves the region from AWS_REGION.
func TestRegionRule_RealDetection(t *testing.T) {
	t.Cleanup(func() {
		context.ResetContext()
		translator.ResetMessages()
	})
	testutil.IsolateAWSSharedConfigEnv(t)
	t.Setenv("AWS_REGION", "eu-west-1")

	testCases := map[string]struct {
		input          string
		wantRegion     string
		wantRegionType string
	}{
		"AgentRegionBeatsEnv": {input: `{"region": "us-east-1"}`, wantRegion: "us-east-1", wantRegionType: config.RegionTypeAgentConfigJson},
		"EnvRegion":           {input: `{}`, wantRegion: "eu-west-1", wantRegionType: config.RegionTypeCredsMap},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			translator.ResetMessages()
			context.ResetContext()
			context.CurrentContext().SetMode(config.ModeOnPrem)
			context.CurrentContext().SetCredentials(map[string]string{})

			var input any
			require.NoError(t, json.Unmarshal([]byte(testCase.input), &input))

			r := new(Region)
			r.ApplyRule(input)

			assert.Equal(t, testCase.wantRegion, Global_Config.Region)
			assert.Equal(t, testCase.wantRegionType, Global_Config.RegionType)
			assert.Empty(t, translator.ErrorMessages)
		})
	}
}
