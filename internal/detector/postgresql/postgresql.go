// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package postgresql

import (
	"context"
	"log/slog"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/postgresql/extract"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
)

const (
	exeName = "postgres"
)

type postgresqlDetector struct {
	logger        *slog.Logger
	portExtractor detector.PortExtractor
}

var _ detector.ProcessDetector = (*postgresqlDetector)(nil)

// NewDetector creates a new process detector that identifies PostgreSQL processes.
func NewDetector(logger *slog.Logger) detector.ProcessDetector {
	return &postgresqlDetector{
		logger:        logger,
		portExtractor: extract.NewPortExtractor(),
	}
}

// Detect identifies PostgreSQL processes and returns metadata.
// Only detects the main postgres process, not worker processes.
func (d *postgresqlDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	exe, err := process.ExeWithContext(ctx)
	if err != nil {
		return nil, err
	}

	base := util.BaseExe(exe)
	if base != exeName {
		return nil, detector.ErrIncompatibleDetector
	}

	// Check if this is the main postgres process or a worker
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return nil, err
	}

	if len(args) > 0 && strings.HasPrefix(strings.TrimSpace(args[0]), exeName+":") {
		return nil, detector.ErrIncompatibleDetector
	}

	d.logger.Debug("PostgreSQL process detected", "pid", process.PID())

	md := &detector.Metadata{
		Name:       "postgresql",
		Categories: []detector.Category{detector.CategoryPostgreSQL},
	}

	port, err := d.portExtractor.Extract(ctx, process)
	if err != nil {
		return nil, err
	}
	md.Status = detector.StatusReady
	md.TelemetryPort = port

	return md, nil
}
