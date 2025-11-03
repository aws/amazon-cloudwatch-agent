// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package detector

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadata_RoundTrip(t *testing.T) {
	testCases := map[string]struct {
		input   Metadata
		want    string
		wantErr bool
	}{
		"FullMetadata": {
			input: Metadata{
				Categories: []Category{CategoryJVM, CategoryTomcat},
				Name:       "test-app",
				Attributes: map[string]string{
					"key": "value",
				},
				TelemetryPort: 8080,
				Status:        StatusReady,
			},
			want: `{"categories":["JVM","TOMCAT"],"name":"test-app","attributes":{"key":"value"},"telemetry_port":8080,"status":"READY"}`,
		},
		"MinimalMetadata": {
			input: Metadata{
				Categories: []Category{CategoryJVM},
				Status:     StatusNeedsSetupJmxPort,
			},
			want: `{"categories":["JVM"],"status":"NEEDS_SETUP/JMX_PORT"}`,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(testCase.input)
			if testCase.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.JSONEq(t, testCase.want, string(data))

			var got Metadata
			err = json.Unmarshal(data, &got)
			assert.NoError(t, err)
			assert.Equal(t, testCase.input, got)
		})
	}
}
