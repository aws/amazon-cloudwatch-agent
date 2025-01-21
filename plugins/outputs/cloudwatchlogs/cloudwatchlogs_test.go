// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"testing"

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
				LogGroupName:   "G1",
				LogStreamName:  "S1",
				AccessKey:      "access_key",
				SecretKey:      "secret_key",
				pusherStopChan: make(chan struct{}),
				cwDests:        make(map[Target]*cwDest),
			}
			dest := c.CreateDest(testCase.cfgLogGroup, testCase.cfgLogStream, testCase.cfgLogRetention, testCase.cfgLogClass, testCase.cfgTailerSrc).(*cwDest)
			require.Equal(t, testCase.expectedLogGroup, dest.pusher.Group)
			require.Equal(t, testCase.expectedLogStream, dest.pusher.Stream)
			require.Equal(t, testCase.expectedLogGroupRetention, dest.pusher.Retention)
			require.Equal(t, testCase.expectedLogClass, dest.pusher.Class)
			require.Equal(t, testCase.expectedTailerSrc, dest.pusher.logSrc)
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
	// Given the same log group, log stream, same retention, and logClass
	d1 := c.CreateDest("FILENAME", "", -1, util.InfrequentAccessLogGroupClass, nil)
	d2 := c.CreateDest("FILENAME", "", -1, util.InfrequentAccessLogGroupClass, nil)

	// Then the destination for cloudwatchlogs endpoint would be the same
	require.Equal(t, d1, d2)
}
