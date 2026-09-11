// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"sync"
	"testing"
	"time"
)

// ---- terminatingSignalReceived atomic flag ---------------------------

func TestTerminatingSignalReceived_DefaultUnset(t *testing.T) {
	t.Cleanup(func() { terminatingSignalReceived.Store(false) })
	if terminatingSignalReceived.Load() {
		t.Fatal("terminatingSignalReceived was true before any Store()")
	}
}

func TestTerminatingSignalReceived_SetLoad(t *testing.T) {
	t.Cleanup(func() { terminatingSignalReceived.Store(false) })
	terminatingSignalReceived.Store(true)
	if !terminatingSignalReceived.Load() {
		t.Fatal("terminatingSignalReceived was false after Store(true)")
	}
}

func TestTerminatingSignalReceived_Race(t *testing.T) {
	t.Cleanup(func() { terminatingSignalReceived.Store(false) })
	const N = 64
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				terminatingSignalReceived.Store(true)
			} else {
				_ = terminatingSignalReceived.Load()
			}
		}(i)
	}
	wg.Wait()
}

// ---- handleTerminatingSignalDispatch ---------------------------------

// withRequestSCMStopFn temporarily overrides requestSCMStopFn and resets
// the terminatingSignalReceived flag for a test.
func withRequestSCMStopFn(t *testing.T, fn func() bool) {
	t.Helper()
	old := requestSCMStopFn
	requestSCMStopFn = fn
	terminatingSignalReceived.Store(false)
	t.Cleanup(func() {
		requestSCMStopFn = old
		terminatingSignalReceived.Store(false)
	})
}

// When requestSCMStop returns false (no SCM path), dispatch must set the
// fallback flag so handleTerminatingSignal can os.Exit later.
func TestDispatch_SCMUnavailable_SetsFallbackFlag(t *testing.T) {
	withRequestSCMStopFn(t, func() bool { return false })
	stopCh := make(chan struct{})
	handleTerminatingSignalDispatch(stopCh, 5*time.Millisecond)
	if !terminatingSignalReceived.Load() {
		t.Fatal("expected fallback flag set when SCM path is unavailable")
	}
}

// When requestSCMStop returns true and close(stop) fires before the
// timeout, dispatch must NOT set the fallback flag (SCM path OK).
func TestDispatch_SCMSuccess_NoFallbackFlag(t *testing.T) {
	withRequestSCMStopFn(t, func() bool { return true })
	stopCh := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(stopCh)
	}()
	handleTerminatingSignalDispatch(stopCh, 500*time.Millisecond)
	if terminatingSignalReceived.Load() {
		t.Fatal("expected NO fallback flag when SCM path completes cleanly")
	}
}

// When requestSCMStop returns true but close(stop) never fires within the
// timeout, dispatch must set the fallback flag.
func TestDispatch_SCMAcceptedButTimeout_SetsFallbackFlag(t *testing.T) {
	withRequestSCMStopFn(t, func() bool { return true })
	stopCh := make(chan struct{}) // never closed
	handleTerminatingSignalDispatch(stopCh, 20*time.Millisecond)
	if !terminatingSignalReceived.Load() {
		t.Fatal("expected fallback flag set after SCM path timeout")
	}
}

// ---- runComplete channel + wait -------------------------------------

// resetRunComplete restores a fresh channel + Once so tests can exercise
// signalRunComplete and waitRunComplete in isolation.
func resetRunComplete() {
	runCompleteChan = make(chan struct{})
	runCompleteOnce = sync.Once{}
}

// signalRunComplete must be idempotent (close of a closed channel would
// panic without sync.Once).
func TestSignalRunComplete_Idempotent(t *testing.T) {
	t.Cleanup(resetRunComplete)
	resetRunComplete()

	signalRunComplete()
	signalRunComplete() // must not panic

	select {
	case <-runCompleteChan:
	default:
		t.Fatal("runCompleteChan not closed after signalRunComplete()")
	}
}

// waitRunComplete must return promptly once signalRunComplete is called.
func TestWaitRunComplete_ReturnsWhenSignaled(t *testing.T) {
	t.Cleanup(resetRunComplete)
	resetRunComplete()
	oldTimeout := runCompleteTimeout
	runCompleteTimeout = 500 * time.Millisecond
	t.Cleanup(func() { runCompleteTimeout = oldTimeout })

	go func() {
		time.Sleep(10 * time.Millisecond)
		signalRunComplete()
	}()

	start := time.Now()
	waitRunComplete()
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Fatalf("waitRunComplete took %v; expected quick return after signal", elapsed)
	}
}

// waitRunComplete must fall through after runCompleteTimeout when no
// signal arrives, so the OS is not held indefinitely.
func TestWaitRunComplete_TimesOut(t *testing.T) {
	t.Cleanup(resetRunComplete)
	resetRunComplete()
	oldTimeout := runCompleteTimeout
	runCompleteTimeout = 20 * time.Millisecond
	t.Cleanup(func() { runCompleteTimeout = oldTimeout })

	start := time.Now()
	waitRunComplete()
	elapsed := time.Since(start)
	if elapsed < 15*time.Millisecond {
		t.Fatalf("waitRunComplete returned in %v; expected ~20ms timeout", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("waitRunComplete took %v; expected ~20ms timeout", elapsed)
	}
}
