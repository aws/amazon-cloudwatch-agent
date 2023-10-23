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
