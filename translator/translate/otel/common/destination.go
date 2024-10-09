// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import "go.opentelemetry.io/collector/confmap"

const (
	DefaultDestination = ""
)

var (
	metricsDestinationsKey = ConfigKey(MetricsKey, MetricsDestinationsKey)
)

func GetMetricsDestinations(conf *confmap.Conf) []string {
	var destinations []string
	if conf.IsSet(ConfigKey(metricsDestinationsKey, CloudWatchKey)) {
		destinations = append(destinations, CloudWatchKey)
	}
	if conf.IsSet(ConfigKey(metricsDestinationsKey, AMPKey)) {
		destinations = append(destinations, AMPKey)
	}
	if conf.IsSet(MetricsKey) && len(destinations) == 0 {
		destinations = append(destinations, DefaultDestination)
	}
	return destinations
}
