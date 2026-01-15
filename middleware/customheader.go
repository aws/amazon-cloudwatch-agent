// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"fmt"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type CustomHeaderFinalizeMiddleware struct {
	Name    string
	Headers map[string]string
}

var _ middleware.FinalizeMiddleware = (*CustomHeaderFinalizeMiddleware)(nil)

func (m *CustomHeaderFinalizeMiddleware) ID() string {
	return m.Name
}

func (m *CustomHeaderFinalizeMiddleware) HandleFinalize(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return middleware.FinalizeOutput{}, middleware.Metadata{}, fmt.Errorf("unrecognized transport type %T", in.Request)
	}
	for k, v := range m.Headers {
		req.Header.Set(k, v)
	}
	return next.HandleFinalize(ctx, in)
}
