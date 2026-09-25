// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package main

import (
	"errors"
	"testing"

	winsvc "golang.org/x/sys/windows/svc"
)

// ---- fake scmManager / scmService for unit tests --------------------

type fakeSCM struct {
	listErr      error
	names        []string
	openFn       func(name string) (scmService, error)
	disconnected bool
}

func (f *fakeSCM) Disconnect() error { f.disconnected = true; return nil }
func (f *fakeSCM) ListServices() ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.names, nil
}
func (f *fakeSCM) OpenService(name string) (scmService, error) {
	if f.openFn != nil {
		return f.openFn(name)
	}
	return nil, errors.New("no openFn")
}

type fakeSvc struct {
	pid        uint32
	queryErr   error
	controlErr error
	closed     bool
	controlCmd winsvc.Cmd
}

func (s *fakeSvc) Close() error { s.closed = true; return nil }
func (s *fakeSvc) Query() (winsvc.Status, error) {
	if s.queryErr != nil {
		return winsvc.Status{}, s.queryErr
	}
	return winsvc.Status{ProcessId: s.pid}, nil
}
func (s *fakeSvc) Control(cmd winsvc.Cmd) (winsvc.Status, error) {
	s.controlCmd = cmd
	if s.controlErr != nil {
		return winsvc.Status{}, s.controlErr
	}
	return winsvc.Status{State: winsvc.StopPending}, nil
}

// withSeams swaps in test doubles for the Windows-only seams and restores
// them (and resets terminatingSignalReceived) on cleanup. Pass nil to
// leave a seam at its default.
func withSeams(t *testing.T,
	exit func(int),
	isSvc func() bool,
	pid func() uint32,
	connect func() (scmManager, error),
) {
	t.Helper()
	oldExit, oldSvc, oldPid, oldConn := exitFunc, isWinService, ownProcessID, scmConnectFunc
	if exit != nil {
		exitFunc = exit
	}
	if isSvc != nil {
		isWinService = isSvc
	}
	if pid != nil {
		ownProcessID = pid
	}
	if connect != nil {
		scmConnectFunc = connect
	}
	t.Cleanup(func() {
		exitFunc, isWinService, ownProcessID, scmConnectFunc = oldExit, oldSvc, oldPid, oldConn
		terminatingSignalReceived.Store(false)
	})
}

// ---- findOwnSCMServiceName -------------------------------------------

func TestFindOwnSCMServiceName_MatchByPID(t *testing.T) {
	svcs := map[string]*fakeSvc{
		"Foo":                   {pid: 100},
		"Bar":                   {pid: 200},
		"AmazonCloudWatchAgent": {pid: 3968},
		"Baz":                   {pid: 400},
	}
	m := &fakeSCM{
		names:  []string{"Foo", "Bar", "AmazonCloudWatchAgent", "Baz"},
		openFn: func(name string) (scmService, error) { return svcs[name], nil },
	}
	withSeams(t, nil, nil, func() uint32 { return 3968 }, nil)

	name, err := findOwnSCMServiceName(m)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "AmazonCloudWatchAgent" {
		t.Fatalf("expected AmazonCloudWatchAgent, got %q", name)
	}
	if !svcs["Foo"].closed || !svcs["Bar"].closed || !svcs["AmazonCloudWatchAgent"].closed {
		t.Errorf("expected Close() on every visited service")
	}
	if svcs["Baz"].closed {
		t.Errorf("walker should have stopped at match; Baz was not visited")
	}
}

func TestFindOwnSCMServiceName_NoMatch(t *testing.T) {
	m := &fakeSCM{
		names:  []string{"Foo", "Bar"},
		openFn: func(_ string) (scmService, error) { return &fakeSvc{pid: 999}, nil },
	}
	withSeams(t, nil, nil, func() uint32 { return 3968 }, nil)

	name, err := findOwnSCMServiceName(m)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "" {
		t.Fatalf("expected empty, got %q", name)
	}
}

func TestFindOwnSCMServiceName_ListServicesError(t *testing.T) {
	sentinel := errors.New("list err")
	m := &fakeSCM{listErr: sentinel}
	withSeams(t, nil, nil, nil, nil)

	name, err := findOwnSCMServiceName(m)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel err, got %v", err)
	}
	if name != "" {
		t.Fatalf("expected empty on error, got %q", name)
	}
}

func TestFindOwnSCMServiceName_SkipsOpenErrors(t *testing.T) {
	svcs := map[string]*fakeSvc{"AmazonCloudWatchAgent": {pid: 3968}}
	m := &fakeSCM{
		names: []string{"Denied", "AmazonCloudWatchAgent"},
		openFn: func(name string) (scmService, error) {
			if name == "Denied" {
				return nil, errors.New("access denied")
			}
			return svcs[name], nil
		},
	}
	withSeams(t, nil, nil, func() uint32 { return 3968 }, nil)

	name, err := findOwnSCMServiceName(m)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "AmazonCloudWatchAgent" {
		t.Fatalf("expected AmazonCloudWatchAgent, got %q", name)
	}
}

func TestFindOwnSCMServiceName_SkipsQueryErrors(t *testing.T) {
	failing := &fakeSvc{queryErr: errors.New("query err")}
	target := &fakeSvc{pid: 3968}
	m := &fakeSCM{
		names: []string{"QueryFails", "AmazonCloudWatchAgent"},
		openFn: func(name string) (scmService, error) {
			if name == "QueryFails" {
				return failing, nil
			}
			return target, nil
		},
	}
	withSeams(t, nil, nil, func() uint32 { return 3968 }, nil)

	name, err := findOwnSCMServiceName(m)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "AmazonCloudWatchAgent" {
		t.Fatalf("expected AmazonCloudWatchAgent, got %q", name)
	}
	if !failing.closed {
		t.Errorf("failing service should still be Close()d after Query err")
	}
}

// ---- requestSCMStop --------------------------------------------------

func TestRequestSCMStop_NotAService(t *testing.T) {
	withSeams(t, nil, func() bool { return false }, nil, nil)
	if requestSCMStop() {
		t.Fatal("expected false when not running as a Windows service")
	}
}

func TestRequestSCMStop_ConnectFail(t *testing.T) {
	sentinel := errors.New("connect err")
	withSeams(t, nil,
		func() bool { return true }, nil,
		func() (scmManager, error) { return nil, sentinel },
	)
	if requestSCMStop() {
		t.Fatal("expected false on SCM Connect failure")
	}
}

func TestRequestSCMStop_NoMatchingService(t *testing.T) {
	m := &fakeSCM{
		names:  []string{"Foo"},
		openFn: func(_ string) (scmService, error) { return &fakeSvc{pid: 999}, nil },
	}
	withSeams(t, nil,
		func() bool { return true },
		func() uint32 { return 3968 },
		func() (scmManager, error) { return m, nil },
	)
	if requestSCMStop() {
		t.Fatal("expected false when no SCM service matches our PID")
	}
	if !m.disconnected {
		t.Error("Disconnect() should have been called")
	}
}

func TestRequestSCMStop_ControlSTOPFail(t *testing.T) {
	target := &fakeSvc{pid: 3968, controlErr: errors.New("control err")}
	m := &fakeSCM{
		names:  []string{"AmazonCloudWatchAgent"},
		openFn: func(_ string) (scmService, error) { return target, nil },
	}
	withSeams(t, nil,
		func() bool { return true },
		func() uint32 { return 3968 },
		func() (scmManager, error) { return m, nil },
	)
	if requestSCMStop() {
		t.Fatal("expected false when Control(Stop) fails")
	}
}

func TestRequestSCMStop_Success(t *testing.T) {
	target := &fakeSvc{pid: 3968}
	m := &fakeSCM{
		names:  []string{"AmazonCloudWatchAgent"},
		openFn: func(_ string) (scmService, error) { return target, nil },
	}
	withSeams(t, nil,
		func() bool { return true },
		func() uint32 { return 3968 },
		func() (scmManager, error) { return m, nil },
	)
	if !requestSCMStop() {
		t.Fatal("expected requestSCMStop success")
	}
	if target.controlCmd != winsvc.Stop {
		t.Errorf("expected winsvc.Stop, got %v", target.controlCmd)
	}
	if !target.closed {
		t.Error("Close() should have been called on the target service")
	}
	if !m.disconnected {
		t.Error("Disconnect() should have been called on the manager")
	}
}

// ---- handleTerminatingSignal -----------------------------------------

func TestHandleTerminatingSignal_FlagUnset_NoExit(t *testing.T) {
	called := false
	withSeams(t, func(int) { called = true }, func() bool { return true }, nil, nil)
	terminatingSignalReceived.Store(false)
	handleTerminatingSignal()
	if called {
		t.Fatal("exitFunc should not have been called with flag unset")
	}
}

func TestHandleTerminatingSignal_NotAService_NoExit(t *testing.T) {
	called := false
	withSeams(t, func(int) { called = true }, func() bool { return false }, nil, nil)
	terminatingSignalReceived.Store(true)
	handleTerminatingSignal()
	if called {
		t.Fatal("exitFunc should not have been called when not a service")
	}
}

func TestHandleTerminatingSignal_FlagSetAndService_Exits(t *testing.T) {
	var exitCode int
	called := false
	withSeams(t, func(code int) { exitCode = code; called = true }, func() bool { return true }, nil, nil)
	terminatingSignalReceived.Store(true)
	handleTerminatingSignal()
	if !called {
		t.Fatal("exitFunc should have been called")
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}
