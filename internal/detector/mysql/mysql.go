// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mysql

import (
	"context"
	"log/slog"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/mysql/extract"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
)

const (
	exeName = "mysqld"
)

type mysqlDetector struct {
	logger        *slog.Logger
	portExtractor detector.PortExtractor
}

var _ detector.ProcessDetector = (*mysqlDetector)(nil)

// NewDetector creates a new process detector that identifies MySQL processes.
func NewDetector(logger *slog.Logger) detector.ProcessDetector {
	return &mysqlDetector{
		logger:        logger,
		portExtractor: extract.NewPortExtractor(),
	}
}

// Detect identifies MySQL server processes and returns metadata.
func (d *mysqlDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	exe, err := process.ExeWithContext(ctx)
	if err != nil {
		return nil, err
	}

	base := util.BaseExe(exe)
	if base != exeName {
		return nil, detector.ErrIncompatibleDetector
	}

	d.logger.Debug("MySQL process detected", "pid", process.PID())

	md := &detector.Metadata{
		Name:       "mysql",
		Categories: []detector.Category{detector.CategoryMySQL},
	}

	port, err := d.portExtractor.Extract(ctx, process)
	if err != nil {
		return nil, err
	}
	md.Status = detector.StatusReady
	md.TelemetryPort = port

	return md, nil
}
