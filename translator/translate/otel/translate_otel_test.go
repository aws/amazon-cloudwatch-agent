// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/agent"
)

func TestTranslator(t *testing.T) {
	agent.Global_Config.Region = "us-east-1"
	ot := NewTranslator()
	testCases := map[string]struct {
		input           interface{}
		wantErrContains string
	}{
		"WithInvalidConfig": {
			input:           "",
			wantErrContains: "invalid json config",
		},
		"WithEmptyConfig": {
			input:           map[string]interface{}{},
			wantErrContains: "no valid pipelines",
		},
		"WithoutReceivers": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			wantErrContains: "no valid pipelines",
		},
		"WithMinimalConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ot.Translate(testCase.input, "linux")
			if testCase.wantErrContains != "" {
				require.Error(t, err)
				require.Nil(t, got)
				t.Log(err)
				require.ErrorContains(t, err, testCase.wantErrContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
			}
		})
	}
}
