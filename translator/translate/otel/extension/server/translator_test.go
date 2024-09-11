// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/extension/server"
)

func TestTranslate(t *testing.T) {
	testCases := map[string]struct {
		input map[string]interface{}
		want  *server.Config
	}{
		"DefaultConfig": {
			input: map[string]interface{}{},
			want:  &server.Config{ListenAddress: defaultListenAddr},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator().(*translator)
			assert.Equal(t, "server", tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
