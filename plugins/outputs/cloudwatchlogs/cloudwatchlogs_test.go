// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"testing"

	"github.com/influxdata/telegraf/plugins/outputs"
)

func TestCreateDest(t *testing.T) {
	// Test filename as log group name
	c := outputs.Outputs["cloudwatchlogs"]().(*CloudWatchLogs)
	c.LogStreamName = "STREAM"

	d0 := c.CreateDest("GROUP", "OTHER_STREAM", -1).(*cwDest)
	if d0.pusher.Group != "GROUP" || d0.pusher.Stream != "OTHER_STREAM" {
		t.Errorf("Wrong target for the created cwDest: %s/%s, expecting GROUP/OTHER_STREAM", d0.pusher.Group, d0.pusher.Stream)
	}

	d1 := c.CreateDest("FILENAME", "", -1).(*cwDest)
	if d1.pusher.Group != "FILENAME" || d1.pusher.Stream != "STREAM" {
		t.Errorf("Wrong target for the created cwDest: %s/%s, expecting FILENAME/STREAM", d1.pusher.Group, d1.pusher.Stream)
	}

	d2 := c.CreateDest("FILENAME", "", -1).(*cwDest)

	if d1 != d2 {
		t.Errorf("Create dest with the same name should return the same cwDest")
	}

	d3 := c.CreateDest("ANOTHERFILE", "", -1).(*cwDest)
	if d1 == d3 {
		t.Errorf("Different file name should result in different cwDest")
	}

	c.LogGroupName = "G1"
	c.LogStreamName = "S1"

	d := c.CreateDest("", "", -1).(*cwDest)

	if d.pusher.Group != "G1" || d.pusher.Stream != "S1" {
		t.Errorf("Empty create dest should return dest to default group and stream, %v/%v found", d.pusher.Group, d.pusher.Stream)
	}
}
