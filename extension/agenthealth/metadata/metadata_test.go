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
		{input: "obs_jvm", want: true},
		{input: "obs_tomcat", want: true},
		{input: "obs_kafka_broker", want: true},
		{input: "obs_nvidia_gpu", want: true},
		{input: "obs_ec2_health", want: true},
		{input: "unsupported_value", want: false},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.want, IsSupported(testCase.input))
	}
}
