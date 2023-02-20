// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"strconv"
	"testing"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm/sdkmetricsdataplane"
)

func TestEmpty(t *testing.T) {
	ring := newRecordRing(10000)
	if !ring.empty() {
		t.Errorf("Expected new record ring to be empty")
	}

	record := &sdkmetricsdataplane.SdkMonitoringRecord{}

	ring.pushFront(record)
	if ring.empty() {
		t.Errorf("Expected newly-pushed record ring to not be empty")
	}

	ring.popFront()
	if !ring.empty() {
		t.Errorf("Expected cleared record ring to be empty")
	}
}

func TestDifferentRecordSizes(t *testing.T) {
	// by only populating the samples field, we make the estimated size
	// closely match a single controllable value
	cases := []struct {
		sizeLimit        int64
		sampleSizes      []int64
		expectedVersions []string
	}{
		{
			sizeLimit:   5000,
			sampleSizes: []int64{3000, 2000},
		},
		{
			sizeLimit:   10000,
			sampleSizes: []int64{4000, 4000, 4000, 4000},
		},
		{
			sizeLimit:   21000,
			sampleSizes: []int64{6000, 5000, 5000, 5000, 5000, 4000},
		},
		{
			sizeLimit:   31000,
			sampleSizes: []int64{11000, 4000, 4000, 4000, 4000, 4000},
		},
		{
			sizeLimit:   17000,
			sampleSizes: []int64{4000, 4000, 4000, 4000},
		},
	}

	for _, c := range cases {
		ring := newRecordRing(c.sizeLimit)

		actualSizes := []int64{}

		for i, size := range c.sampleSizes {
			samples := string(make([]rune, size))
			version := strconv.Itoa(i)
			record := &sdkmetricsdataplane.SdkMonitoringRecord{
				CompressedEventSamples: &samples,
				Version:                &version,
			}

			ring.pushFront(record)
			actualSizes = append(actualSizes, estimateRecordSize(record))
		}

		remainingSize := int64(0)
		startRemainingIndex := 0
		for ri := len(actualSizes) - 1; ri >= 0; ri-- {
			if remainingSize+actualSizes[ri] > c.sizeLimit {
				startRemainingIndex = ri + 1
				break
			}

			remainingSize += actualSizes[ri]
		}

		index := 0
		for ri := len(c.sampleSizes) - 1; ri >= startRemainingIndex; ri-- {
			record := ring.popFront()

			if record == nil {
				t.Errorf("Expected record in position %v, but was empty", index)
			}

			recordSize := estimateRecordSize(record)
			if recordSize != actualSizes[ri] {
				t.Errorf("Expected record of size %v in position %v, but saw size %v instead", actualSizes[ri], index, recordSize)
			}

			versionNumber, _ := strconv.Atoi(*record.Version)
			if versionNumber != ri {
				t.Errorf("Expected records to be popped in LIFO order")
			}

			index++
		}

		if !ring.empty() {
			t.Errorf("Expected record ring to be empty when drained of its size capacity")
		}
	}
}
