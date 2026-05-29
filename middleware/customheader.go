// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"fmt"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type CustomHeaderMiddleware struct {
	MiddlewareID string
	Fn           func() map[string]string
}

var _ middleware.BuildMiddleware = (*CustomHeaderMiddleware)(nil)

func NewCustomHeaderMiddleware(middlewareID string, headers map[string]string) *CustomHeaderMiddleware {
	return &CustomHeaderMiddleware{
		MiddlewareID: middlewareID,
		Fn:           func() map[string]string { return headers },
	}
}

func (m *CustomHeaderMiddleware) ID() string {
	return m.MiddlewareID
}

func (m *CustomHeaderMiddleware) HandleBuild(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return middleware.BuildOutput{}, middleware.Metadata{}, fmt.Errorf("unrecognized transport type %T", in.Request)
	}
	for k, v := range m.Fn() {
		req.Header.Set(k, v)
	}
	return next.HandleBuild(ctx, in)
}
