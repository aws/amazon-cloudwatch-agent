// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logger

import (
	"io"
	"os"
	"path/filepath"

	telegraf_logger "github.com/influxdata/telegraf/logger"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	LogTargetLumberjack = "lumberjack"
)

// Implement the LoggerCreator interface so it can be registered with telegraf_logger.
type lumberjackLogCreator struct {
}

func (t *lumberjackLogCreator) CreateLogger(config telegraf_logger.LogConfig) (io.Writer, error) {
	var writer, defaultWriter io.Writer
	defaultWriter = os.Stderr
	if config.Logfile != "" {
		os.MkdirAll(filepath.Dir(config.Logfile), 0755)
		// The codes below should not change, because the retention information has already been published to public doc.
		writer = &lumberjack.Logger{
			Filename:   config.Logfile,
			MaxSize:    100,
			MaxBackups: 5,
			MaxAge:     7,
			Compress:   true,
		}
	} else {
		writer = defaultWriter
	}

	return telegraf_logger.NewTelegrafWriter(writer), nil
}

func init() {
	llc := &lumberjackLogCreator{}
	telegraf_logger.RegisterLogger(LogTargetLumberjack, llc)
}
