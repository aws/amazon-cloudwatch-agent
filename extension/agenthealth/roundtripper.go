// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"net/http"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/google/uuid"
)

var _ http.RoundTripper = (*roundTripper)(nil)

type roundTripper struct {
	base             http.RoundTripper
	requestHandlers  []awsmiddleware.RequestHandler
	responseHandlers []awsmiddleware.ResponseHandler
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := awsmiddleware.SetRequestID(req.Context(), uuid.NewString())
	ctx = awsmiddleware.SetOperationName(ctx, req.URL.Path)
	req = req.WithContext(ctx)
	req.Header.Del("User-Agent")
	for _, h := range rt.requestHandlers {
		h.HandleRequest(ctx, req)
	}
	resp, err := rt.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	for _, h := range rt.responseHandlers {
		h.HandleResponse(ctx, resp)
	}
	return resp, nil
}
