// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package main

import (
	"log"
	"os"

	winsvc "golang.org/x/sys/windows/svc"
	winmgr "golang.org/x/sys/windows/svc/mgr"
)

// Workaround for Windows OS-shutdown deadlock (Event 6008 / Kernel-Power 41):
// at OS shutdown, csrss delivers CTRL_SHUTDOWN (=> SIGTERM) to the collector
// but SERVICE_CONTROL_SHUTDOWN goes to the launcher, not us. kardianos'
// windowsService.Run only watches the SCM channel, so svc.Run stays blocked.
// requestSCMStop finds this process's SCM entry by matching ProcessId (since
// kardianos' config Name -- "telegraf" by default -- is not in SCM) and
// issues STOP so kardianos' normal path can drive svc.Run to return.

// scmManager / scmService: minimal interfaces so tests can substitute fakes.
type scmManager interface {
	Disconnect() error
	ListServices() ([]string, error)
	OpenService(name string) (scmService, error)
}
type scmService interface {
	Close() error
	Query() (winsvc.Status, error)
	Control(cmd winsvc.Cmd) (winsvc.Status, error)
}

type realSCMManager struct{ m *winmgr.Mgr }

func (r *realSCMManager) Disconnect() error               { return r.m.Disconnect() }
func (r *realSCMManager) ListServices() ([]string, error) { return r.m.ListServices() }
func (r *realSCMManager) OpenService(name string) (scmService, error) {
	s, err := r.m.OpenService(name)
	if err != nil {
		return nil, err
	}
	return &realSCMService{s: s}, nil
}

type realSCMService struct{ s *winmgr.Service }

func (r *realSCMService) Close() error                                  { return r.s.Close() }
func (r *realSCMService) Query() (winsvc.Status, error)                 { return r.s.Query() }
func (r *realSCMService) Control(cmd winsvc.Cmd) (winsvc.Status, error) { return r.s.Control(cmd) }

// Test seams (single-writer via t.Cleanup, single-reader in shutdown path).
var (
	exitFunc       = os.Exit
	isWinService   = func() bool { return windowsRunAsService() }
	ownProcessID   = func() uint32 { return uint32(os.Getpid()) }
	scmConnectFunc = func() (scmManager, error) {
		m, err := winmgr.Connect()
		if err != nil {
			return nil, err
		}
		return &realSCMManager{m: m}, nil
	}
)

// findOwnSCMServiceName returns the SCM service whose ProcessId matches
// this process. Empty string if not found. Entries that cannot be opened
// or queried are skipped.
func findOwnSCMServiceName(m scmManager) (string, error) {
	names, err := m.ListServices()
	if err != nil {
		return "", err
	}
	myPID := ownProcessID()
	for _, name := range names {
		s, err := m.OpenService(name)
		if err != nil {
			continue
		}
		status, err := s.Query()
		s.Close()
		if err == nil && status.ProcessId == myPID {
			return name, nil
		}
	}
	return "", nil
}

// requestSCMStop issues SERVICE_CONTROL_STOP against this process's own
// SCM entry. Returns true on success.
func requestSCMStop() bool {
	if !isWinService() {
		return false
	}
	m, err := scmConnectFunc()
	if err != nil {
		log.Printf("W! Windows service: SCM Connect failed: %v", err)
		return false
	}
	defer m.Disconnect()

	scmName, err := findOwnSCMServiceName(m)
	if err != nil {
		log.Printf("W! Windows service: SCM ListServices failed: %v", err)
		return false
	}
	if scmName == "" {
		log.Printf("W! Windows service: no SCM service found for own PID=%d", ownProcessID())
		return false
	}

	s, err := m.OpenService(scmName)
	if err != nil {
		log.Printf("W! Windows service: SCM OpenService(%q) failed: %v", scmName, err)
		return false
	}
	defer s.Close()
	if _, err := s.Control(winsvc.Stop); err != nil {
		log.Printf("W! Windows service: SCM Control(Stop) on %q failed: %v", scmName, err)
		return false
	}
	log.Printf("I! Windows service: SCM Control(Stop) accepted on %q; waiting for prg.Stop", scmName)
	return true
}

// handleTerminatingSignal is the fallback: exit the process iff the flag is
// set (SCM path unavailable) and we're running as a Windows service.
func handleTerminatingSignal() {
	if !terminatingSignalReceived.Load() {
		return
	}
	if !isWinService() {
		return
	}
	log.Println("I! Windows service: SCM stop path unavailable; exiting to release svc.Run")
	exitFunc(0)
}
