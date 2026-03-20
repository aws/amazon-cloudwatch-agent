// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type RoundTripFunc func(*http.Request) (*http.Response, error)

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRoundTripper_RequestHandlersCalled(t *testing.T) {
	mockHandler := &awsmiddleware.MockHandler{}
	mockHandler.On("ID").Maybe().Return("test")
	mockHandler.On("Position").Maybe().Return(awsmiddleware.After)
	mockHandler.On("HandleRequest", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		assert.NotEmpty(t, awsmiddleware.GetRequestID(ctx))
		assert.Equal(t, "/v1/metrics", awsmiddleware.GetOperationName(ctx))
	}).Return()

	base := RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	rt := &roundTripper{
		base:            base,
		requestHandlers: []awsmiddleware.RequestHandler{mockHandler},
	}

	req := httptest.NewRequest(http.MethodPost, "http://localhost/v1/metrics", nil)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockHandler.AssertCalled(t, "HandleRequest", mock.Anything, mock.Anything)
}

func TestRoundTripper_ResponseHandlersCalled(t *testing.T) {
	mockHandler := &awsmiddleware.MockHandler{}
	mockHandler.On("ID").Maybe().Return("test")
	mockHandler.On("Position").Maybe().Return(awsmiddleware.After)
	mockHandler.On("HandleRequest", mock.Anything, mock.Anything).Return()
	mockHandler.On("HandleResponse", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		assert.NotEmpty(t, awsmiddleware.GetRequestID(ctx))
		assert.Equal(t, "/v1/metrics", awsmiddleware.GetOperationName(ctx))
	}).Return()

	base := RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	rt := &roundTripper{
		base:             base,
		requestHandlers:  []awsmiddleware.RequestHandler{mockHandler},
		responseHandlers: []awsmiddleware.ResponseHandler{mockHandler},
	}

	req := httptest.NewRequest(http.MethodPost, "http://localhost/v1/metrics", nil)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	mockHandler.AssertCalled(t, "HandleRequest", mock.Anything, mock.Anything)
	mockHandler.AssertCalled(t, "HandleResponse", mock.Anything, mock.Anything)
}

func TestRoundTripper_ErrorSkipsResponseHandlers(t *testing.T) {
	mockHandler := &awsmiddleware.MockHandler{}
	mockHandler.On("ID").Maybe().Return("test")
	mockHandler.On("Position").Maybe().Return(awsmiddleware.After)
	mockHandler.On("HandleRequest", mock.Anything, mock.Anything).Return()

	base := RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	rt := &roundTripper{
		base:             base,
		requestHandlers:  []awsmiddleware.RequestHandler{mockHandler},
		responseHandlers: []awsmiddleware.ResponseHandler{mockHandler},
	}

	req := httptest.NewRequest(http.MethodPost, "http://localhost/v1/metrics", nil)
	resp, err := rt.RoundTrip(req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	mockHandler.AssertCalled(t, "HandleRequest", mock.Anything, mock.Anything)
	mockHandler.AssertNotCalled(t, "HandleResponse", mock.Anything, mock.Anything)
}
