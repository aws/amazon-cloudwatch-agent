// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kafkabroker

import (
	"context"
	"log/slog"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/kafkabroker/extract"
)

const (
	// brokerMetadataName is the value placed in the metadata name field for a Kafka broker.
	brokerMetadataName = "Kafka Broker"
)

type kafkaBrokerDetector struct {
	logger              *slog.Logger
	attributesExtractor detector.Extractor[map[string]string]
}

// NewDetector creates a new process detector that identifies Apache Kafka instances. This detector is meant to be
// used as a sub-detector of the Java detector.
func NewDetector(logger *slog.Logger) detector.ProcessDetector {
	return &kafkaBrokerDetector{
		logger:              logger,
		attributesExtractor: extract.NewAttributesExtractor(logger),
	}
}

// Detect identifies Kafka broker processes by looking at the command-line arguments to find the Kafka main class
// (`kafka.Kafka`). Returns metadata with the KAFKA/BROKER category and Kafka Broker as the name if detected.
func (d *kafkaBrokerDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	attributes, err := d.attributesExtractor.Extract(ctx, process)
	if err != nil {
		d.logger.Debug("Kafka broker not detected for process", "pid", process.PID(), "error", err)
		return nil, detector.ErrIncompatibleDetector
	}

	md := &detector.Metadata{
		Categories: []detector.Category{detector.CategoryKafkaBroker},
		Name:       brokerMetadataName,
	}
	if len(attributes) > 0 {
		md.Attributes = attributes
	}
	return md, nil
}
