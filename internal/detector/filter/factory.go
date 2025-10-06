// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filter

import (
	"log/slog"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/filter/name"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/filter/process"
)

type Filters struct {
	Process ProcessFilters
}

type ProcessFilters struct {
	Pre  detector.ProcessFilter
	Name detector.NameFilter
}

func FromConfig(logger *slog.Logger, cfg Config) Filters {
	return Filters{
		Process: fromProcessConfig(logger, cfg.Process),
	}
}

func fromProcessConfig(logger *slog.Logger, cfg ProcessConfig) ProcessFilters {
	f := ProcessFilters{}
	if cfg.MinUptime > 0 {
		f.Pre = process.NewMinUptimeFilter(logger, cfg.MinUptime)
	}
	if len(cfg.ExcludeNames) > 0 {
		f.Name = name.NewExcludeFilter(cfg.ExcludeNames...)
	}
	return f
}
