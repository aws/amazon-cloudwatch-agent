// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build integration
// +build integration

package util

import (
	"os"
	"testing"
	"time"
	"fmt"
)

const (
	logLineId1       = "foo"
	logLineId2       = "bar"
)

var logLineIds = []string{logLineId1, logLineId2}

func WriteLogs(t *testing.T, filePath string, iterations int) {
	f, err := os.Create(filePath)
	
	if err != nil {
		t.Fatalf("Error occurred creating log file for writing: %v", err)
	}
	
	defer f.Close()
	defer os.Remove(filePath)
	
	t.Logf("Writing %d lines to %s", iterations*len(logLineIds), filePath)
	
	for i := 0; i < iterations; i++ {
		ts := time.Now()
		for _, id := range logLineIds {
			_, err = f.WriteString(fmt.Sprintf("%s - [%s] #%d This is a log line.\n", ts.Format(time.StampMilli), id, i))
			if err != nil {
				// don't need to fatal error here. if a log line doesn't get written, the count
				// when validating the log stream should be incorrect and fail there.
				t.Logf("Error occurred writing log line: %v", err)
			}
		}
		time.Sleep(1 * time.Millisecond)
	}
}