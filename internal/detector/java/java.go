// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package java

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/java/extract"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

const (
	exeName = "java"
)

type javaDetector struct {
	logger        *slog.Logger
	subDetectors  []detector.ProcessDetector
	nameExtractor detector.NameExtractor
	portExtractor detector.PortExtractor
}

var _ detector.ProcessDetector = (*javaDetector)(nil)

func NewDetector(logger *slog.Logger) detector.ProcessDetector {
	return &javaDetector{
		logger:        logger,
		subDetectors:  []detector.ProcessDetector{},
		nameExtractor: extract.NewNameExtractor(logger, collections.NewSet(paths.JMXJarName)),
		portExtractor: extract.JmxPortExtractor,
	}
}

func (d *javaDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	exe, err := process.ExeWithContext(ctx)
	if err != nil {
		return nil, err
	}

	base := util.BaseExe(exe)
	if base != exeName {
		return nil, detector.ErrIncompatibleDetector
	}

	for _, sd := range d.subDetectors {
		var md *detector.Metadata
		md, err = sd.Detect(ctx, process)
		if err != nil {
			continue
		}
		return md, nil
	}

	name, err := d.nameExtractor.Extract(ctx, process)
	if err != nil {
		d.logger.Debug(fmt.Sprintf("failed to extract java process name: %v", err))
		return nil, err
	}
	md := &detector.Metadata{
		Categories: []detector.Category{detector.CategoryJVM},
		Name:       name,
	}
	port, err := d.portExtractor.Extract(ctx, process)
	if err != nil {
		md.Status = detector.StatusNeedsSetupJmxPort
	} else {
		md.Status = detector.StatusReady
		md.TelemetryPort = port
	}
	return md, nil
}
