// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"time"
)

// AggregationRecordFilter is used to filter metrics based off a given function.
type AggregationRecordFilter func(AggregationRecord) bool

// filterPrior is used to filter any metrics that are older than the cutoff time.
type filterPrior struct {
	cutoff time.Time
}

func (filter filterPrior) Filter(record AggregationRecord) bool {
	return filter.cutoff.Before(record.Expiry)
}
