// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// See shutdown_signal_windows.go for the workaround's purpose.

// stopWaitTimeout bounds how long the signal goroutine waits for prg.Stop
// after requestSCMStop is accepted. Declared as var for testability.
var stopWaitTimeout = 30 * time.Second

// terminatingSignalReceived flags that the SCM STOP path was NOT taken;
// handleTerminatingSignal then os.Exit(0)s so svc.Run cannot block OS
// shutdown. Accessed atomically.
var terminatingSignalReceived atomic.Bool

// requestSCMStopFn is a test seam over requestSCMStop.
var requestSCMStopFn = requestSCMStop

// handleTerminatingSignalDispatch is called by the signal goroutine on a
// non-SIGHUP terminating signal. Tries SCM STOP; falls back to setting the
// flag on failure or timeout.
func handleTerminatingSignalDispatch(stopCh <-chan struct{}, timeout time.Duration) {
	if requestSCMStopFn() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-stopCh:
		case <-timer.C:
			log.Println("W! Windows service: SCM stop did not deliver in time; forcing fallback")
			terminatingSignalReceived.Store(true)
		}
	} else {
		terminatingSignalReceived.Store(true)
	}
}

// runCompleteChan is closed by signalRunComplete when (*program).run
// finishes. main() waits on this via waitRunComplete so otelcol.Shutdown
// and any pending flushes can complete before the process exits.
// Declared as var so tests can substitute a fresh channel.
var runCompleteChan = make(chan struct{})

// runCompleteOnce guards close(runCompleteChan).
var runCompleteOnce sync.Once

// runCompleteTimeout bounds waitRunComplete so a stuck teardown cannot
// indefinitely hold up the OS.
var runCompleteTimeout = 30 * time.Second

// signalRunComplete unblocks waitRunComplete. Safe to call multiple times.
func signalRunComplete() {
	runCompleteOnce.Do(func() { close(runCompleteChan) })
}

// waitRunComplete blocks until signalRunComplete or runCompleteTimeout,
// whichever comes first. Logs a warning on timeout so the OS is not
// held indefinitely.
func waitRunComplete() {
	select {
	case <-runCompleteChan:
	case <-time.After(runCompleteTimeout):
		log.Printf("W! Windows service: waited %v for shutdown teardown to complete; exiting anyway", runCompleteTimeout)
	}
}
