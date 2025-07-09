// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
)

type MetricIdentity struct {
	name string
	tags map[string]any
}

func (mi *MetricIdentity) getKey() string {
	b := new(bytes.Buffer)
	keys := slices.Sorted(maps.Keys(mi.tags))
	for _, k := range keys {
		_, _ = fmt.Fprintf(b, "%s=%s,", k, mi.tags[k])
	}
	// We assume there won't be same metricName+tags with different metricType, so that it is not necessary to add metricType into uniqKey.
	_, _ = fmt.Fprintf(b, "metricName=%s,", mi.name)
	return b.String()
}
