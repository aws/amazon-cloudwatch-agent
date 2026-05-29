// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"net/http"
	"testing"

	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomHeaderFinalizeMiddleware_ID(t *testing.T) {
	middleware := &CustomHeaderMiddleware{
		MiddlewareID: "TestMiddleware",
	}

	assert.Equal(t, "TestMiddleware", middleware.ID())
}

func TestCustomHeaderFinalizeMiddleware_HandleFinalize(t *testing.T) {
	testCases := []struct {
		name            string
		headers         map[string]string
		existingHeaders map[string]string
		expectedHeaders map[string]string
	}{
		{
			name: "add single header",
			headers: map[string]string{
				"X-Custom-Header": "test-value",
			},
			existingHeaders: map[string]string{},
			expectedHeaders: map[string]string{
				"X-Custom-Header": "test-value",
			},
		},
		{
			name: "add multiple headers",
			headers: map[string]string{
				"X-Custom-Header-1": "value1",
				"X-Custom-Header-2": "value2",
			},
			existingHeaders: map[string]string{},
			expectedHeaders: map[string]string{
				"X-Custom-Header-1": "value1",
				"X-Custom-Header-2": "value2",
			},
		},
		{
			name: "override existing header",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			existingHeaders: map[string]string{
				"Content-Type": "text/plain",
			},
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name: "preserve existing headers",
			headers: map[string]string{
				"X-New-Header": "new-value",
			},
			existingHeaders: map[string]string{
				"X-Existing-Header": "existing-value",
			},
			expectedHeaders: map[string]string{
				"X-New-Header":      "new-value",
				"X-Existing-Header": "existing-value",
			},
		},
		{
			name:            "no headers to add",
			headers:         map[string]string{},
			existingHeaders: map[string]string{},
			expectedHeaders: map[string]string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			middleware := NewCustomHeaderMiddleware("TestMiddleware", testCase.headers)

			// Create mock HTTP request
			httpReq, err := http.NewRequest("POST", "https://www.amazon.com", nil)
			require.NoError(t, err)

			// Set existing headers
			for k, v := range testCase.existingHeaders {
				httpReq.Header.Set(k, v)
			}

			// Create smithy HTTP request
			req := &smithyhttp.Request{
				Request: httpReq,
			}

			input := smithymiddleware.BuildInput{
				Request: req,
			}

			// Mock next handler
			nextCalled := false
			next := smithymiddleware.BuildHandlerFunc(func(context.Context, smithymiddleware.BuildInput) (smithymiddleware.BuildOutput, smithymiddleware.Metadata, error) {
				nextCalled = true
				return smithymiddleware.BuildOutput{}, smithymiddleware.Metadata{}, nil
			})

			// Execute middleware
			_, _, err = middleware.HandleBuild(context.Background(), input, next)
			require.NoError(t, err)
			assert.True(t, nextCalled)

			// Verify headers
			for expectedKey, expectedValue := range testCase.expectedHeaders {
				assert.Equal(t, expectedValue, req.Header.Get(expectedKey), "Header %s should have value %s", expectedKey, expectedValue)
			}

			// Verify no unexpected headers were added (only check if we have expected headers)
			if len(testCase.expectedHeaders) > 0 {
				for actualKey := range req.Header {
					if _, expected := testCase.expectedHeaders[actualKey]; !expected {
						// This header was not in our expected set, it must have been an existing header
						_, wasExisting := testCase.existingHeaders[actualKey]
						assert.True(t, wasExisting, "Unexpected header %s found", actualKey)
					}
				}
			}
		})
	}
}

func TestCustomHeaderFinalizeMiddleware_InvalidRequest(t *testing.T) {
	middleware := NewCustomHeaderMiddleware("TestMiddleware", map[string]string{
		"X-Test": "value",
	})

	// Use invalid request type
	input := smithymiddleware.BuildInput{
		Request: "invalid-request-type",
	}

	next := smithymiddleware.BuildHandlerFunc(func(context.Context, smithymiddleware.BuildInput) (smithymiddleware.BuildOutput, smithymiddleware.Metadata, error) {
		t.Fatal("Next handler should not be called")
		return smithymiddleware.BuildOutput{}, smithymiddleware.Metadata{}, nil
	})

	_, _, err := middleware.HandleBuild(context.Background(), input, next)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized transport type")
}

func TestCustomHeaderFinalizeMiddleware_EmptyHeaders(t *testing.T) {
	middleware := &CustomHeaderMiddleware{
		MiddlewareID: "TestMiddleware",
		Fn: func() map[string]string {
			return nil
		},
	}

	// Create mock HTTP request
	httpReq, err := http.NewRequest("POST", "https://www.amazon.com", nil)
	require.NoError(t, err)

	req := &smithyhttp.Request{
		Request: httpReq,
	}

	input := smithymiddleware.BuildInput{
		Request: req,
	}

	nextCalled := false
	next := smithymiddleware.BuildHandlerFunc(func(context.Context, smithymiddleware.BuildInput) (smithymiddleware.BuildOutput, smithymiddleware.Metadata, error) {
		nextCalled = true
		return smithymiddleware.BuildOutput{}, smithymiddleware.Metadata{}, nil
	})

	_, _, err = middleware.HandleBuild(context.Background(), input, next)
	require.NoError(t, err)
	assert.True(t, nextCalled)
}
