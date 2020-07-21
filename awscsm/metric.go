// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsmmetrics

import (
	"time"
)

// Metric interface that is used to retrieve the necessary data to construct a
// PutRecord call to the voxdataplane service.
type Metric interface {
	GetFrequencyMetrics() []FrequencyMetric
	GetSEHMetrics() []SEHMetric
	GetTimestamp() time.Time
	GetKeys() map[string]string
	GetSamples() []map[string]interface{}
}
