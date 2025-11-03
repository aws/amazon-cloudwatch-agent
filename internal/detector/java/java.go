// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package java

import (
	"context"
	"errors"
	"log/slog"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/java/extract"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/kafkabroker"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/tomcat"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
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

// NewDetector creates a new process detector that identifies Java applications. It uses specialized sub-detectors
// for known applications to further classify them.
func NewDetector(logger *slog.Logger, nameFilter detector.NameFilter) detector.ProcessDetector {
	return &javaDetector{
		logger: logger,
		subDetectors: []detector.ProcessDetector{
			tomcat.NewDetector(logger),
			kafkabroker.NewDetector(logger),
		},
		nameExtractor: extract.NewNameExtractor(logger, nameFilter),
		portExtractor: extract.NewPortExtractor(),
	}
}

// Detect identifies Java processes and attempts to further classify them using sub-detectors. If no sub-detector
// matches, it falls back to generic Java process detection using JAR/class name extraction. All detected processes
// are tagged with the JVM category and include JMX port detection.
func (d *javaDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	exe, err := process.ExeWithContext(ctx)
	if err != nil {
		return nil, err
	}

	base := util.BaseExe(exe)
	if base != exeName {
		return nil, detector.ErrIncompatibleDetector
	}

	md := &detector.Metadata{}
	for _, sd := range d.subDetectors {
		var detected *detector.Metadata
		detected, err = sd.Detect(ctx, process)
		if err == nil && detected != nil {
			md = detected
			break
		}
	}
	md.Categories = append([]detector.Category{detector.CategoryJVM}, md.Categories...)
	if md.Name == "" {
		md.Name, err = d.nameExtractor.Extract(ctx, process)
		if err != nil {
			if errors.Is(err, detector.ErrSkipProcess) {
				d.logger.Debug("Java process skipped", "pid", process.PID(), "err", err)
			} else {
				d.logger.Debug("Failed to extract Java process name", "pid", process.PID(), "err", err)
			}
			return nil, err
		}
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
