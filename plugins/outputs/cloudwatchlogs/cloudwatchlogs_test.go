// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCreateDestination would create different destination for cloudwatchlogs endpoint based on the log group, log stream,
// and log group's retention
func TestCreateDestination(t *testing.T) {

	testCases := map[string]struct {
		cfgLogGroup               string
		cfgLogStream              string
		cfgLogRetention           int
		expectedLogGroup          string
		expectedLogStream         string
		expectedLogGroupRetention int
	}{
		"WithTomlGroupStream": {
			cfgLogGroup:               "",
			cfgLogStream:              "",
			cfgLogRetention:           -1,
			expectedLogGroup:          "G1",
			expectedLogStream:         "S1",
			expectedLogGroupRetention: -1,
		},
		"WithOverrideGroupStream": {
			cfgLogGroup:               "Group5",
			cfgLogStream:              "Stream5",
			cfgLogRetention:           -1,
			expectedLogGroup:          "Group5",
			expectedLogStream:         "Stream5",
			expectedLogGroupRetention: -1,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			c := &CloudWatchLogs{
				LogGroupName:   "G1",
				LogStreamName:  "S1",
				AccessKey:      "access_key",
				SecretKey:      "secret_key",
				pusherStopChan: make(chan struct{}),
				cwDests:        make(map[Target]*cwDest),
			}
			dest := c.CreateDest(testCase.cfgLogGroup, testCase.cfgLogStream, testCase.cfgLogRetention).(*cwDest)
			require.Equal(t, testCase.expectedLogGroup, dest.pusher.Group)
			require.Equal(t, testCase.expectedLogStream, dest.pusher.Stream)
		})
	}
}

func TestDuplicateDestination(t *testing.T) {
	c := &CloudWatchLogs{
		AccessKey:      "access_key",
		SecretKey:      "secret_key",
		cwDests:        make(map[Target]*cwDest),
		pusherStopChan: make(chan struct{}),
	}
	// Given the same log group, log stream and same retention
	d1 := c.CreateDest("FILENAME", "", -1)
	d2 := c.CreateDest("FILENAME", "", -1)

	// Then the destination for cloudwatchlogs endpoint would be the same
	require.Equal(t, d1, d2)
}
