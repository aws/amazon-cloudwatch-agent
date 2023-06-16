// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package handlers

import (
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomHeaderHandler(t *testing.T) {
	handler := NewCustomHeaderHandler("name", "test")
	httpReq, err := http.NewRequest("", "", nil)
	require.NoError(t, err)

	req := &request.Request{HTTPRequest: httpReq}
	handler.Fn(req)
	assert.Equal(t, "test", req.HTTPRequest.Header.Get("name"))
}

func TestNewDynamicCustomHeaderHandler(t *testing.T) {
	content := "1"
	fn := func() string {
		return content
	}
	handler := NewDynamicCustomHeaderHandler("name", fn)
	httpReq, err := http.NewRequest("", "", nil)
	require.NoError(t, err)
	req := &request.Request{HTTPRequest: httpReq}

	handler.Fn(req)
	assert.Equal(t, "1", req.HTTPRequest.Header.Get("name"))

	content = "2"
	handler.Fn(req)
	assert.Equal(t, "2", req.HTTPRequest.Header.Get("name"))
}
