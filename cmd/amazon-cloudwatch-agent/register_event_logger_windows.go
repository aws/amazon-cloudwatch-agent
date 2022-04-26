// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package main

import (
	lumberjack "github.com/aws/amazon-cloudwatch-agent/logger"
	"github.com/influxdata/telegraf/logger"
)

func RegisterEventLogger() error {
	// When in service mode, register eventlog target and setup default logging to eventlog

	e := logger.RegisterEventLogger(LogTargetEventLog)
	if e != nil {
		return e
	}
	logger.SetupLogging(logger.LogConfig{LogTarget: lumberjack.LogTargetLumberjack})
	return nil
}