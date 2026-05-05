// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuild(t *testing.T) {
	testCases := []struct {
		key   string
		value string
		want  string
	}{
		{key: "ObservabilitySolution", value: "ec2_health", want: "obs_ec2_health"},
		{key: "ObservabilitySolution", value: "JVM", want: "obs_jvm"},
		{key: "OBSERVABILITYSOLUTION", value: "TOMCAT", want: "obs_tomcat"},
		{key: "observabilitysolution", value: "kafka_broker", want: "obs_kafka_broker"},
		{key: "ObservabilitySolution", value: "NVIDIA_GPU", want: "obs_nvidia_gpu"},
		{key: "ObservabilitySolution", value: "jvm_ec2", want: "obs_jvm_ec2"},
		{key: "ObservabilitySolution", value: "tomcat_ec2", want: "obs_tomcat_ec2"},
		{key: "ObservabilitySolution", value: "nvidia_gpu_ec2", want: "obs_nvidia_gpu_ec2"},
		{key: "ObservabilitySolution", value: "log_collection", want: "obs_log_collection"},
		{key: "ObservabilitySolution", value: "application_signals", want: "obs_application_signals"},
		{key: "ObservabilitySolution", value: "emf", want: "obs_emf"},
		{key: "unsupported", value: "Value", want: "unsupported_value"},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.want, Build(testCase.key, testCase.value))
	}
}

func TestIsSupported(t *testing.T) {
	testCases := []struct {
		input string
		want  bool
	}{
		{input: "obs_ec2_health", want: true},
		{input: "obs_jvm", want: true},
		{input: "obs_tomcat", want: true},
		{input: "obs_kafka_broker", want: true},
		{input: "obs_kafka_consumer", want: true},
		{input: "obs_kafka_producer", want: true},
		{input: "obs_nvidia_gpu", want: true},
		{input: "obs_jvm_ec2", want: true},
		{input: "obs_tomcat_ec2", want: true},
		{input: "obs_nvidia_gpu_ec2", want: true},
		{input: "obs_log_collection", want: true},
		{input: "obs_windows_events", want: true},
		{input: "obs_xray_traces", want: true},
		{input: "obs_otel_traces", want: true},
		{input: "obs_application_signals", want: true},
		{input: "obs_statsd", want: true},
		{input: "obs_collectd", want: true},
		{input: "obs_prometheus", want: true},
		{input: "obs_otel_metrics", want: true},
		{input: "obs_emf", want: true},
		{input: "obs_process_monitoring", want: true},
		{input: "unsupported_value", want: false},
		{input: "obs_unknown", want: false},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.want, IsSupported(testCase.input))
	}
}
