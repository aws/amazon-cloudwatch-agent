// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kafka

import (
	"context"
	"log/slog"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

const (
	// brokerClassName is the main Java class name for the Kafka broker. This is what is normally run when starting a
	// Kafka broker.
	brokerClassName = "kafka.Kafka"
	// brokerMetadataName is the value placed in the metadata name field for a Kafka broker.
	brokerMetadataName = "Kafka Broker"
)

type kafkaDetector struct {
	logger       *slog.Logger
	subDetectors []detector.ProcessDetector
}

// NewDetector creates a new process detector that identifies Apache Kafka instances. This detector is meant to be
// used as a sub-detector of the Java detector.
func NewDetector(logger *slog.Logger) detector.ProcessDetector {
	return &kafkaDetector{
		logger: logger,
		subDetectors: []detector.ProcessDetector{
			newCmdlineDetector(logger),
		},
	}
}

// Detect identifies Kafka broker processes by looking at the command-line arguments to find the Kafka main class
// (`kafka.Kafka`). Returns metadata with the Kafka/Broker category and Kafka Broker as the name if detected.
func (d *kafkaDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	var md *detector.Metadata
	for _, sd := range d.subDetectors {
		var err error
		md, err = sd.Detect(ctx, process)
		if err != nil {
			continue
		}
		break
	}

	if md == nil {
		return nil, detector.ErrIncompatibleDetector
	}
	return md, nil
}

type cmdlineDetector struct {
	logger *slog.Logger
}

func newCmdlineDetector(logger *slog.Logger) detector.ProcessDetector {
	return &cmdlineDetector{logger: logger}
}

func (d *cmdlineDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var isKafkaBroker bool
	for i := len(args) - 1; i >= 0; i-- {
		if strings.Contains(args[i], brokerClassName) {
			isKafkaBroker = true
			break
		}
	}

	if !isKafkaBroker {
		return nil, detector.ErrIncompatibleDetector
	}

	return &detector.Metadata{
		Categories: []detector.Category{detector.CategoryKafkaBroker},
		Name:       brokerMetadataName,
	}, nil
}
