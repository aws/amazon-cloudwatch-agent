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
	ValueApplicationSignals   = "application_signals"
	ValueCollectd             = "collectd"
	ValueEC2Health            = "ec2_health"
	ValueEC2HealthWindows     = "ec2_health_windows"
	ValueEMF                  = "emf"
	ValueJVM                  = "jvm"
	ValueJVMEC2               = "jvm_ec2"
	ValueKafkaBroker          = "kafka_broker"
	ValueKafkaConsumer        = "kafka_consumer"
	ValueKafkaProducer        = "kafka_producer"
	ValueLogCollection        = "log_collection"
	ValueNVIDIA               = "nvidia_gpu"
	ValueNVIDIAEC2            = "nvidia_gpu_ec2"
	ValueOtelMetrics          = "otel_metrics"
	ValueOtelTraces           = "otel_traces"
	ValueProcessMonitoring    = "process_monitoring"
	ValuePrometheus           = "prometheus"
	ValueStatsd               = "statsd"
	ValueTomcat               = "tomcat"
	ValueTomcatEC2            = "tomcat_ec2"
	ValueWindowsEvents        = "windows_events"
	ValueXrayTraces           = "xray_traces"

	shortKeyObservabilitySolutions = "obs"
	separator                      = "_"
)

var (
	aliases         = map[Metadata]Metadata{}
	shortKeyMapping = map[string]string{
		strings.ToLower(KeyObservabilitySolutions): shortKeyObservabilitySolutions,
	}
	supportedMetadata = collections.NewSet(
		Build(KeyObservabilitySolutions, ValueApplicationSignals),
		Build(KeyObservabilitySolutions, ValueCollectd),
		Build(KeyObservabilitySolutions, ValueEC2Health),
		Build(KeyObservabilitySolutions, ValueEC2HealthWindows),
		Build(KeyObservabilitySolutions, ValueEMF),
		Build(KeyObservabilitySolutions, ValueJVM),
		Build(KeyObservabilitySolutions, ValueKafkaBroker),
		Build(KeyObservabilitySolutions, ValueKafkaConsumer),
		Build(KeyObservabilitySolutions, ValueKafkaProducer),
		Build(KeyObservabilitySolutions, ValueLogCollection),
		Build(KeyObservabilitySolutions, ValueNVIDIA),
		Build(KeyObservabilitySolutions, ValueOtelMetrics),
		Build(KeyObservabilitySolutions, ValueOtelTraces),
		Build(KeyObservabilitySolutions, ValueProcessMonitoring),
		Build(KeyObservabilitySolutions, ValuePrometheus),
		Build(KeyObservabilitySolutions, ValueStatsd),
		Build(KeyObservabilitySolutions, ValueTomcat),
		Build(KeyObservabilitySolutions, ValueWindowsEvents),
		Build(KeyObservabilitySolutions, ValueXrayTraces),
	)
)

func init() {
	aliases[Build(KeyObservabilitySolutions, ValueJVMEC2)] = Build(KeyObservabilitySolutions, ValueJVM)
	aliases[Build(KeyObservabilitySolutions, ValueTomcatEC2)] = Build(KeyObservabilitySolutions, ValueTomcat)
	aliases[Build(KeyObservabilitySolutions, ValueNVIDIAEC2)] = Build(KeyObservabilitySolutions, ValueNVIDIA)
}

func IsSupported(m Metadata) bool {
	if base, ok := aliases[m]; ok {
		m = base
	}
	return supportedMetadata.Contains(m)
}

// Resolve returns the base value for an alias, or the input unchanged.
func Resolve(m Metadata) Metadata {
	if base, ok := aliases[m]; ok {
		return base
	}
	return m
}

func Build(key, value string) Metadata {
	key = strings.ToLower(key)
	if shortKey, ok := shortKeyMapping[key]; ok {
		key = shortKey
	}
	return key + separator + strings.ToLower(value)
}
