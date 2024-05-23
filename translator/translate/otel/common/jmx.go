// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import "go.opentelemetry.io/collector/confmap"

func GetJmxMap(conf *confmap.Conf, index int) map[string]any {
	var got map[string]any
	switch v := conf.Get(JmxConfigKey).(type) {
	case []any:
		if index != -1 && len(v) > index {
			got = v[index].(map[string]any)
		}
	case map[string]any:
		got = v
	}
	return got
}

func GetMeasurements(m map[string]any) []string {
	var results []string
	if measurements, ok := m[MeasurementKey].([]any); ok {
		for _, measurement := range measurements {
			if s, ok := measurement.(string); ok {
				results = append(results, s)
			}
		}
	}
	return results
}
