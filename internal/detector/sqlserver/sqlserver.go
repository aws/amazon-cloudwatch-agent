// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package sqlserver

import (
	"context"
	"log/slog"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/sqlserver/extract"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
)

const (
	exeName = "sqlservr"
)

type sqlserverDetector struct {
	logger        *slog.Logger
	portExtractor detector.PortExtractor
}

var _ detector.ProcessDetector = (*sqlserverDetector)(nil)

// NewDetector creates a new process detector that identifies SQL Server processes.
func NewDetector(logger *slog.Logger) detector.ProcessDetector {
	return &sqlserverDetector{
		logger:        logger,
		portExtractor: extract.NewPortExtractor(),
	}
}

// Detect identifies SQL Server processes and returns metadata.
func (d *sqlserverDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	exe, err := process.ExeWithContext(ctx)
	if err != nil {
		return nil, err
	}

	base := util.BaseExe(exe)
	if base != exeName {
		return nil, detector.ErrIncompatibleDetector
	}

	d.logger.Debug("SQL Server process detected", "pid", process.PID())

	md := &detector.Metadata{
		Name:       "sqlserver",
		Categories: []detector.Category{detector.CategorySQLServer},
	}

	port, err := d.portExtractor.Extract(ctx, process)
	if err != nil {
		return nil, err
	}
	md.Status = detector.StatusReady
	md.TelemetryPort = port

	return md, nil
}
