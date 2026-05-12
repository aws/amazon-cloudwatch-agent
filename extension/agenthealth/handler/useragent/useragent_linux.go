//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"go.opentelemetry.io/collector/otelcol"
)

const (
	flagJournaldEnabled = "jd_enabled"
)

func (ua *userAgent) setJournaldFeatureFlags(otelCfg *otelcol.Config) {
	for _, cfg := range otelCfg.Receivers {
		if _, ok := cfg.(*journaldreceiver.JournaldConfig); ok {
			ua.feature.Add(flagJournaldEnabled)
			return
		}
	}
}
