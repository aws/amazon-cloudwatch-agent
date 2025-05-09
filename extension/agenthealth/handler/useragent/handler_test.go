// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"context"
	"net/http"
	"testing"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestUserAgentHandler(t *testing.T) {
	t.Setenv(envconfig.CWAGENT_USER_AGENT, "FirstUA")
	ua := newUserAgent()
	handler := newHandler(ua, true)
	assert.Equal(t, handlerID, handler.ID())
	assert.Equal(t, awsmiddleware.After, handler.Position())
	req, err := http.NewRequest("", "localhost", nil)
	require.NoError(t, err)
	handler.HandleRequest(context.Background(), req)
	assert.Equal(t, "FirstUA", req.Header.Get(headerKeyUserAgent))
	t.Setenv(envconfig.CWAGENT_USER_AGENT, "SecondUA")
	ua.notify()
	handler.HandleRequest(context.Background(), req)
	assert.Equal(t, "SecondUA FirstUA", req.Header.Get(headerKeyUserAgent))
}

func TestParseUserAgent(t *testing.T) {
	testCases := map[string]struct {
		userAgent      string
		expectedPlugin string
		expectedHeader string
	}{
		"WithEBS": {
			userAgent:      "EBS",
			expectedPlugin: metricPluginEBS,
			expectedHeader: "inputs:(" + metricPluginEBS + ")",
		},
		"WithoutEBS": {
			userAgent:      "banana rainforest",
			expectedPlugin: "",
			expectedHeader: "",
		},
		"MultipleParsing": {
			userAgent:      "banana EBS rainforest",
			expectedPlugin: metricPluginEBS,
			expectedHeader: "inputs:(" + metricPluginEBS + ")",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ua := newUserAgent()
			handler := newHandler(ua, true)

			handler.ParseUserAgent(tc.userAgent)

			if tc.expectedPlugin != "" {
				_, exists := handler.detectedPlugins.Load(tc.expectedPlugin)
				assert.True(t, exists, "plugin should be detected")

				assert.True(t, ua.inputs.Contains(tc.expectedPlugin),
					"plugin should be added to inputs")

				assert.Contains(t, handler.Header(), tc.expectedPlugin)
			} else {
				assert.Equal(t, 0, len(ua.inputs))
			}

			handler.ParseUserAgent(tc.userAgent)
			if tc.expectedPlugin != "" {
				assert.Equal(t, 1, len(ua.inputs), "plugin should only be added once")
				assert.True(t, ua.inputs.Contains(tc.expectedPlugin))
			}
		})
	}
}
