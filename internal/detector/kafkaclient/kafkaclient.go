// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kafkaclient

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/common"
)

const (
	kafkaClientsLibraryName = "kafka-clients"
)

type kafkaClientDetector struct {
	logger       *slog.Logger
	subDetectors []detector.ProcessDetector
}

func NewDetector(logger *slog.Logger) detector.ProcessDetector {
	return &kafkaClientDetector{
		logger: logger,
		subDetectors: []detector.ProcessDetector{
			&classPathDetector{logger: logger},
			&loadedJarDetector{logger: logger},
		},
	}
}

func (d *kafkaClientDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	for _, sd := range d.subDetectors {
		detected, err := sd.Detect(ctx, process)
		if err == nil && detected != nil {
			return detected, nil
		}
	}
	d.logger.Debug("Kafka client not detected for process", "pid", process.PID())
	return nil, detector.ErrIncompatibleDetector
}

type classPathDetector struct {
	logger *slog.Logger
}

func (d *classPathDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return nil, err
	}
	var isNextArgClassPath bool
	for _, arg := range args {
		if isNextArgClassPath {
			if strings.Contains(arg, kafkaClientsLibraryName) {
				return &detector.Metadata{Categories: []detector.Category{detector.CategoryKafkaClient}}, nil
			}
			isNextArgClassPath = false
			continue
		}
		isNextArgClassPath = isClassPathFlag(arg)
	}
	return nil, detector.ErrIncompatibleDetector
}

func isClassPathFlag(arg string) bool {
	return arg == "-cp" || arg == "-classpath"
}

type loadedJarDetector struct {
	logger *slog.Logger
}

func (d *loadedJarDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	fds, err := process.OpenFilesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	for _, fd := range fds {
		path := strings.TrimSuffix(fd.Path, " (deleted)")
		if !strings.HasSuffix(path, common.ExtJAR) {
			continue
		}
		if strings.Contains(filepath.Base(path), kafkaClientsLibraryName) {
			return &detector.Metadata{Categories: []detector.Category{detector.CategoryKafkaClient}}, nil
		}
	}
	return nil, detector.ErrIncompatibleDetector
}
