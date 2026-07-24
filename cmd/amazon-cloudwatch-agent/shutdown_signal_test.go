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
