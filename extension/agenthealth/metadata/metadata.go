// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metadata

import (
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

type Metadata = string

const (
	KeyObservabilitySolutions = "ObservabilitySolution"
	ValueEC2Health            = "ec2_health"
	ValueJVM                  = "jvm"
	ValueTomcat               = "tomcat"
	ValueKafkaBroker          = "kafka_broker"
	ValueKafkaConsumer        = "kafka_consumer"
	ValueKafkaProducer        = "kafka_producer"
	ValueNVIDIA               = "nvidia_gpu"

	shortKeyObservabilitySolutions = "obs"
	separator                      = "_"
)

var (
	supportedMetadata = collections.NewSet(
		Build(KeyObservabilitySolutions, ValueEC2Health),
		Build(KeyObservabilitySolutions, ValueJVM),
		Build(KeyObservabilitySolutions, ValueTomcat),
		Build(KeyObservabilitySolutions, ValueKafkaBroker),
		Build(KeyObservabilitySolutions, ValueKafkaConsumer),
		Build(KeyObservabilitySolutions, ValueKafkaProducer),
		Build(KeyObservabilitySolutions, ValueNVIDIA),
	)
	shortKeyMapping = map[string]string{
		strings.ToLower(KeyObservabilitySolutions): shortKeyObservabilitySolutions,
	}
)

func IsSupported(m Metadata) bool {
	return supportedMetadata.Contains(m)
}

// Build finds any short key mappings and then adds them to the value.
func Build(key, value string) Metadata {
	key = strings.ToLower(key)
	if shortKey, ok := shortKeyMapping[key]; ok {
		key = shortKey
	}
	return key + separator + strings.ToLower(value)
}
