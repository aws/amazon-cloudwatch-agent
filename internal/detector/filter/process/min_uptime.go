// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package process

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

type minUptimeFilter struct {
	logger    *slog.Logger
	minUptime time.Duration
	timeSince func(time.Time) time.Duration
}

func NewMinUptimeFilter(logger *slog.Logger, minUptime time.Duration) detector.ProcessFilter {
	return &minUptimeFilter{
		logger:    logger,
		minUptime: minUptime,
		timeSince: time.Since,
	}
}

func (f *minUptimeFilter) ShouldInclude(ctx context.Context, process detector.Process) bool {
	createTime, err := process.CreateTimeWithContext(ctx)
	if err != nil {
		f.logger.Debug("Unable to get create time for process", "process", process.PID(), "err", err)
		return true // if unknown create time, allow it through
	}
	return f.timeSince(time.UnixMilli(createTime)) >= f.minUptime
}
