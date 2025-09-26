// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tomcat

import (
	"context"
	"log/slog"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/tomcat/extract"
)

type tomcatDetector struct {
	logger        *slog.Logger
	nameExtractor detector.NameExtractor
}

// NewDetector creates a new process detector that identifies Apache Tomcat instances. This detector is meant to be
// used as a sub-detector of the Java detector.
func NewDetector(logger *slog.Logger) detector.ProcessDetector {
	return &tomcatDetector{
		logger:        logger,
		nameExtractor: extract.NewNameExtractor(logger),
	}
}

// Detect identifies Apache Tomcat processes by locating the CATALINA_BASE or CATALINA_HOME directories of the process.
// It searches process command-line arguments and environment variables for these Tomcat-specific properties. Returns
// metadata with the Tomcat category and the discovered directory path as the name.
func (d *tomcatDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	name, err := d.nameExtractor.Extract(ctx, process)
	if err != nil {
		d.logger.Debug("Tomcat not detected for process", "pid", process.PID(), "error", err)
		return nil, detector.ErrIncompatibleDetector
	}

	d.logger.Debug("Detected Tomcat directory", "pid", process.PID(), "path", name)

	md := &detector.Metadata{
		Categories: []detector.Category{detector.CategoryTomcat},
		Name:       name,
	}
	return md, nil
}
