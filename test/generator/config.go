// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package generator

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type Config struct {
	Interval    time.Duration
	Annotations map[string]interface{}
	Metadata    map[string]map[string]interface{}
	Attributes  []attribute.KeyValue
}
