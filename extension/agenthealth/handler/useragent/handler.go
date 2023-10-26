// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"context"
	"net/http"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.uber.org/atomic"
)

const (
	handlerID          = "cloudwatchagent.UserAgent"
	headerKeyUserAgent = "User-Agent"
)

type userAgentHandler struct {
	userAgent          UserAgent
	isUsageDataEnabled bool
	header             *atomic.String
}

var _ awsmiddleware.RequestHandler = (*userAgentHandler)(nil)

func (uah *userAgentHandler) ID() string {
	return handlerID
}

func (uah *userAgentHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

// HandleRequest prepends the User-Agent header with the CloudWatch Agent's
// user agent string.
func (uah *userAgentHandler) HandleRequest(_ context.Context, r *http.Request) {
	newHeader := uah.Header()
	current := r.Header.Get(headerKeyUserAgent)
	if current != "" {
		newHeader += separator + current
	}
	r.Header.Set(headerKeyUserAgent, newHeader)
}

func (uah *userAgentHandler) Header() string {
	return uah.header.Load()
}

func (uah *userAgentHandler) refreshHeader() {
	uah.header.Store(uah.userAgent.Header(uah.isUsageDataEnabled))
}

func newHandler(userAgent UserAgent, isUsageDataEnabled bool) *userAgentHandler {
	handler := &userAgentHandler{
		userAgent:          userAgent,
		header:             &atomic.String{},
		isUsageDataEnabled: isUsageDataEnabled,
	}
	handler.refreshHeader()
	userAgent.Listen(handler.refreshHeader)
	return handler
}

func NewHandler(isUsageDataEnabled bool) awsmiddleware.RequestHandler {
	return newHandler(Get(), isUsageDataEnabled)
}
