// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"regexp"
)

const (
	PROCESSED_NEURON_METRIC_PATTERN = "^(container|node|pod)_(neuroncore_|neurondevice_).*|^node_neuron_.*"
)

type AwsNeuronMetricChecker struct {
}

func NewAwsNeuronMetricChecker() *AwsNeuronMetricChecker {
	return &AwsNeuronMetricChecker{}
}

func (md *AwsNeuronMetricChecker) IsProcessedNeuronMetric(name string) bool {
	matched, err := regexp.MatchString(PROCESSED_NEURON_METRIC_PATTERN, name)
	if err != nil {
		print(err)
		return false
	}
	return matched
}
