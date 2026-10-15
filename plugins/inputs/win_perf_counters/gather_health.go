// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package win_perf_counters

import "fmt"

const (
	// initFailureEscalateAfter is how many consecutive empty scrapes escalate from W! to E!.
	initFailureEscalateAfter = 3
	// initFailureResyncEvery re-emits E! so long-running outages stay visible at default log level.
	initFailureResyncEvery = 10
)

// scrapeOutcome summarizes one Gather pass for logging and error reporting.
type scrapeOutcome struct {
	total   int
	failed  int
	lastErr error
}

func (o scrapeOutcome) allFailed() bool {
	return o.total > 0 && o.failed == o.total && o.lastErr != nil
}

func (o scrapeOutcome) error() error {
	if !o.allFailed() {
		return nil
	}
	return fmt.Errorf("win_perf_counters: all %d counters failed to initialize or collect: %w", o.total, o.lastErr)
}

// nextFailureCount updates consecutive failed-scrape tracking after a Gather pass.
func nextFailureCount(prev int, outcome scrapeOutcome) int {
	if outcome.allFailed() {
		return prev + 1
	}
	return 0
}

// failureLogPrefix chooses W! or E! for a consecutive failure count.
// First failure is W!; E! at the escalate threshold and every resync thereafter.
func failureLogPrefix(consecutive int) string {
	if consecutive <= 0 {
		return ""
	}
	if consecutive == 1 {
		return "W!"
	}
	if consecutive == initFailureEscalateAfter || consecutive%initFailureResyncEvery == 0 {
		return "E!"
	}
	return ""
}
