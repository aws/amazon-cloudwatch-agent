// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "journald", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"logs": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: baseKey,
			},
		},
		"WithJournald": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{},
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
			}
		})
	}
}

func TestNewTranslator(t *testing.T) {
	translator := NewTranslator()
	assert.Equal(t, "journald", translator.ID().String())

	customName := "custom_name"
	translator = NewTranslatorWithName(customName)
	assert.Equal(t, "journald/"+customName, translator.ID().String())
}