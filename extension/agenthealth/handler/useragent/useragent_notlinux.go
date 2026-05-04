//go:build !linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"go.opentelemetry.io/collector/otelcol"
)

func (ua *userAgent) setJournaldFeatureFlags(_ *otelcol.Config) {
	// No-op on non-Linux platforms
}
