// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"sync"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

// TestCreateDestination would create different destination for cloudwatchlogs endpoint based on the log group, log stream,
// and log group's retention
func TestCreateDestination(t *testing.T) {
	testCases := map[string]struct {
		cfgLogGroup               string
		cfgLogStream              string
		cfgLogRetention           int
		cfgLogClass               string
		cfgTailerSrc              logs.LogSrc
		expectedLogGroup          string
		expectedLogStream         string
		expectedLogGroupRetention int
		expectedLogClass          string
		expectedTailerSrc         logs.LogSrc
	}{
		"WithTomlGroupStream": {
			cfgLogGroup:               "",
			cfgLogStream:              "",
			cfgLogRetention:           -1,
			cfgLogClass:               "",
			cfgTailerSrc:              nil,
			expectedLogGroup:          "G1",
			expectedLogStream:         "S1",
			expectedLogGroupRetention: -1,
			expectedTailerSrc:         nil,
		},
		"WithOverrideGroupStreamStandardLogGroup": {
			cfgLogGroup:               "",
			cfgLogStream:              "",
			cfgLogRetention:           -1,
			cfgLogClass:               util.StandardLogGroupClass,
			cfgTailerSrc:              nil,
			expectedLogGroup:          "G1",
			expectedLogStream:         "S1",
			expectedLogGroupRetention: -1,
			expectedLogClass:          util.StandardLogGroupClass,
			expectedTailerSrc:         nil,
		},
		"WithOverrideGroupStreamInfrequentLogGroup": {
			cfgLogGroup:               "Group5",
			cfgLogStream:              "Stream5",
			cfgLogRetention:           -1,
			cfgLogClass:               util.InfrequentAccessLogGroupClass,
			cfgTailerSrc:              nil,
			expectedLogGroup:          "Group5",
			expectedLogStream:         "Stream5",
			expectedLogGroupRetention: -1,
			expectedLogClass:          util.InfrequentAccessLogGroupClass,
			expectedTailerSrc:         nil,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			c := &CloudWatchLogs{
				Log:           testutil.Logger{Name: "test"},
				LogGroupName:  "G1",
				LogStreamName: "S1",
				AccessKey:     "access_key",
				SecretKey:     "secret_key",
				cwDests:       sync.Map{},
			}
			dest := c.CreateDest(testCase.cfgLogGroup, testCase.cfgLogStream, testCase.cfgLogRetention, testCase.cfgLogClass, testCase.cfgTailerSrc).(*cwDest)
			require.Equal(t, testCase.expectedLogGroup, dest.pusher.Group)
			require.Equal(t, testCase.expectedLogStream, dest.pusher.Stream)
			require.Equal(t, testCase.expectedLogGroupRetention, dest.pusher.Retention)
			require.Equal(t, testCase.expectedLogClass, dest.pusher.Class)
			require.Equal(t, testCase.expectedTailerSrc, dest.pusher.EntityProvider)
		})
	}
}

func TestDuplicateDestination(t *testing.T) {
	c := &CloudWatchLogs{
		Log:       testutil.Logger{Name: "test"},
		AccessKey: "access_key",
		SecretKey: "secret_key",
		cwDests:   sync.Map{},
	}
	// Given the same log group, log stream, same retention, and logClass
	d1 := c.CreateDest("FILENAME", "", -1, util.InfrequentAccessLogGroupClass, nil)
	d2 := c.CreateDest("FILENAME", "", -1, util.InfrequentAccessLogGroupClass, nil)

	// Then the destination for cloudwatchlogs endpoint would be the same
	require.Equal(t, d1, d2)
}

// TestSharedRetryerLifecycle verifies that stopping one destination does not affect
// the shared TargetManager's ability to create new targets, and that the shared
// retryer is separate from any destination's retryer.
func TestSharedRetryerLifecycle(t *testing.T) {
	c := &CloudWatchLogs{
		Log:       testutil.Logger{Name: "test"},
		AccessKey: "access_key",
		SecretKey: "secret_key",
		cwDests:   sync.Map{},
	}

	// Create the first destination - this initializes the shared TargetManager
	d1 := c.CreateDest("group1", "stream1", -1, "", nil).(*cwDest)

	// Verify that the shared retryer was created and is separate from d1's retryer
	require.NotNil(t, c.sharedRetryer, "shared retryer should be initialized")
	require.NotNil(t, c.sharedClient, "shared client should be initialized")
	require.NotSame(t, c.sharedRetryer, d1.retryer, "shared retryer should be separate from destination retryer")

	// Stop the first destination (simulates log rotation with auto_removal)
	d1.Stop()

	// Create a second destination - this should not block or fail
	done := make(chan *cwDest, 1)
	go func() {
		d2 := c.CreateDest("group2", "stream2", -1, "", nil).(*cwDest)
		done <- d2
	}()

	select {
	case d2 := <-done:
		require.NotNil(t, d2, "second destination should be created successfully")
		require.NotSame(t, d1, d2, "second destination should be different from first")
		// Clean up
		d2.Stop()
	case <-time.After(5 * time.Second):
		t.Fatal("creating second destination blocked after first destination was stopped - potential deadlock")
	}

	// Clean up
	c.Close()
}
