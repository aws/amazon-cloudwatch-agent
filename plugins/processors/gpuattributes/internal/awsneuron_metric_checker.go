// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"regexp"
)

type AwsNeuronMetricChecker struct {
}

func NewAwsNeuronMetricChecker() *AwsNeuronMetricChecker {
	return &AwsNeuronMetricChecker{}
}

func (md *AwsNeuronMetricChecker) IsProcessedNeuronMetric(name string) bool {
	matched, err := regexp.MatchString("^(container|node|pod)_(neuroncore_|neurondevice_).*|^node_neuron_.*", name)
	if err != nil {
		print(err)
		return false
	}
	return matched
}
