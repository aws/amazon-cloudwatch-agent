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
	middleware := &CustomHeaderFinalizeMiddleware{
		Name: "TestMiddleware",
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			middleware := &CustomHeaderFinalizeMiddleware{
				Name:    "TestMiddleware",
				Headers: tc.headers,
			}

			// Create mock HTTP request
			httpReq, err := http.NewRequest("POST", "https://www.amazon.com", nil)
			require.NoError(t, err)

			// Set existing headers
			for k, v := range tc.existingHeaders {
				httpReq.Header.Set(k, v)
			}

			// Create smithy HTTP request
			req := &smithyhttp.Request{
				Request: httpReq,
			}

			input := smithymiddleware.FinalizeInput{
				Request: req,
			}

			// Mock next handler
			nextCalled := false
			next := smithymiddleware.FinalizeHandlerFunc(func(context.Context, smithymiddleware.FinalizeInput) (smithymiddleware.FinalizeOutput, smithymiddleware.Metadata, error) {
				nextCalled = true
				return smithymiddleware.FinalizeOutput{}, smithymiddleware.Metadata{}, nil
			})

			// Execute middleware
			_, _, err = middleware.HandleFinalize(context.Background(), input, next)
			require.NoError(t, err)
			assert.True(t, nextCalled)

			// Verify headers
			for expectedKey, expectedValue := range tc.expectedHeaders {
				assert.Equal(t, expectedValue, req.Header.Get(expectedKey), "Header %s should have value %s", expectedKey, expectedValue)
			}

			// Verify no unexpected headers were added (only check if we have expected headers)
			if len(tc.expectedHeaders) > 0 {
				for actualKey := range req.Header {
					if _, expected := tc.expectedHeaders[actualKey]; !expected {
						// This header was not in our expected set, it must have been an existing header
						_, wasExisting := tc.existingHeaders[actualKey]
						assert.True(t, wasExisting, "Unexpected header %s found", actualKey)
					}
				}
			}
		})
	}
}

func TestCustomHeaderFinalizeMiddleware_InvalidRequest(t *testing.T) {
	middleware := &CustomHeaderFinalizeMiddleware{
		Name: "TestMiddleware",
		Headers: map[string]string{
			"X-Test": "value",
		},
	}

	// Use invalid request type
	input := smithymiddleware.FinalizeInput{
		Request: "invalid-request-type",
	}

	next := smithymiddleware.FinalizeHandlerFunc(func(context.Context, smithymiddleware.FinalizeInput) (smithymiddleware.FinalizeOutput, smithymiddleware.Metadata, error) {
		t.Fatal("Next handler should not be called")
		return smithymiddleware.FinalizeOutput{}, smithymiddleware.Metadata{}, nil
	})

	_, _, err := middleware.HandleFinalize(context.Background(), input, next)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized transport type")
}

func TestCustomHeaderFinalizeMiddleware_EmptyHeaders(t *testing.T) {
	middleware := &CustomHeaderFinalizeMiddleware{
		Name:    "TestMiddleware",
		Headers: nil, // nil headers map
	}

	// Create mock HTTP request
	httpReq, err := http.NewRequest("POST", "https://www.amazon.com", nil)
	require.NoError(t, err)

	req := &smithyhttp.Request{
		Request: httpReq,
	}

	input := smithymiddleware.FinalizeInput{
		Request: req,
	}

	nextCalled := false
	next := smithymiddleware.FinalizeHandlerFunc(func(context.Context, smithymiddleware.FinalizeInput) (smithymiddleware.FinalizeOutput, smithymiddleware.Metadata, error) {
		nextCalled = true
		return smithymiddleware.FinalizeOutput{}, smithymiddleware.Metadata{}, nil
	})

	_, _, err = middleware.HandleFinalize(context.Background(), input, next)
	require.NoError(t, err)
	assert.True(t, nextCalled)
}
