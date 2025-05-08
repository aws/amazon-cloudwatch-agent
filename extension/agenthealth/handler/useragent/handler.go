// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"context"
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"go.uber.org/atomic"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	handlerID          = "cloudwatchagent.UserAgent"
	headerKeyUserAgent = "User-Agent"

	metricPluginNVME = "cinvme"
)

// MetricPluginMapping defines what user agent string to look for
// and what plugin name to add when found
var metricPluginMapping = map[string]string{
	"NVME": metricPluginNVME,
}

type userAgentHandler struct {
	userAgent          UserAgent
	isUsageDataEnabled bool
	header             *atomic.String
	detectedPlugins    sync.Map
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
	uah.ParseUserAgent(current)
	if current != "" {
		newHeader += separator + current
	}
	log.Printf("current user agent header is: %s, new user agent header is: %s", current, newHeader)
	r.Header.Set(headerKeyUserAgent, newHeader)
}

func (uah *userAgentHandler) Header() string {
	return uah.header.Load()
}

func (uah *userAgentHandler) refreshHeader() {
	uah.header.Store(uah.userAgent.Header(uah.isUsageDataEnabled))
}

func (uah *userAgentHandler) ParseUserAgent(fullUserAgent string) {
	log.Println("parsing user agent for NVME header")
	// Check each defined metric plugin mapping
	for userAgentStr, pluginName := range metricPluginMapping {
		// Skip if we've already detected this plugin
		if _, exists := uah.detectedPlugins.Load(pluginName); exists {
			log.Println("Header exists, skipping creation")
			continue
		}

		// If user agent contains the string we're looking for
		if strings.Contains(fullUserAgent, userAgentStr) {
			uah.detectedPlugins.Store(pluginName, struct{}{})
			// Add plugin to inputs for monitoring
			uah.userAgent.AddInput(pluginName)
			uah.refreshHeader()
			log.Printf("Added %s plugin to user agent string", pluginName)
		}
	}
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
