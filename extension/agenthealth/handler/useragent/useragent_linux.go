//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"go.opentelemetry.io/collector/otelcol"
)

const (
	flagJournaldUnits    = "jd_units"
	flagJournaldPriority = "jd_priority"
	flagJournaldMatches  = "jd_matches"
	flagJournaldFilters  = "jd_filters"

	defaultJournaldPriority = "info"
)

func (ua *userAgent) setJournaldFeatureFlags(otelCfg *otelcol.Config) {
	for _, cfg := range otelCfg.Receivers {
		if journaldCfg, ok := cfg.(*journaldreceiver.JournaldConfig); ok {
			if len(journaldCfg.InputConfig.Units) > 0 {
				ua.feature.Add(flagJournaldUnits)
			}
			if journaldCfg.InputConfig.Priority != "" && journaldCfg.InputConfig.Priority != defaultJournaldPriority {
				ua.feature.Add(flagJournaldPriority)
			}
			if len(journaldCfg.InputConfig.Matches) > 0 {
				ua.feature.Add(flagJournaldMatches)
			}
		}
	}
	for pipelineID, pipeline := range otelCfg.Service.Pipelines {
		if strings.Contains(pipelineID.Name(), "journald") {
			for _, processor := range pipeline.Processors {
				if processor.Type().String() == "filter" {
					ua.feature.Add(flagJournaldFilters)
					return
				}
			}
		}
	}
}
