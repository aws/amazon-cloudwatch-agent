// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"strings"
)

const (
	CONTAINER_NEURON_CORE_METRIC_PREFIX   = "container_neuroncore_"
	NODE_NEURON_CORE_METRIC_PREFIX        = "node_neuroncore_"
	POD_NEURON_CORE_METRIC_PREFIX         = "pod_neuroncore_"
	CONTAINER_NEURON_DEVICE_METRIC_PREFIX = "container_neurondevice_"
	NODE_NEURON_DEVICE_METRIC_PREFIX      = "node_neurondevice_"
	POD_NEURON_DEVICE_METRIC_PREFIX       = "pod_neurondevice_"
	NODE_NEURON_METRIC_PREFIX             = "node_neuron_"
)

type AwsNeuronMetricChecker struct {
}

func NewAwsNeuronMetricChecker() *AwsNeuronMetricChecker {
	return &AwsNeuronMetricChecker{}
}

func (md *AwsNeuronMetricChecker) IsProcessedNeuronMetric(name string) bool {
	switch {
	case strings.HasPrefix(name, CONTAINER_NEURON_CORE_METRIC_PREFIX):
		return true
	case strings.HasPrefix(name, POD_NEURON_CORE_METRIC_PREFIX):
		return true
	case strings.HasPrefix(name, NODE_NEURON_CORE_METRIC_PREFIX):
		return true
	case strings.HasPrefix(name, CONTAINER_NEURON_DEVICE_METRIC_PREFIX):
		return true
	case strings.HasPrefix(name, POD_NEURON_DEVICE_METRIC_PREFIX):
		return true
	case strings.HasPrefix(name, NODE_NEURON_DEVICE_METRIC_PREFIX):
		return true
	case strings.HasPrefix(name, NODE_NEURON_METRIC_PREFIX):
		return true
	default:
		return false
	}
}
