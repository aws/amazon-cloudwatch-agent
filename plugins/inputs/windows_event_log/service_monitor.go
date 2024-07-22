// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package windows_event_log

import (
	"errors"
	"log"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceCheckInterval = 10 * time.Second

	serviceName = "eventlog"
)

var (
	errServiceNotRunning = errors.New("service is not running")
)

type statusChecker interface {
	Query() (svc.Status, error)
}

type serviceMonitor struct {
	listeners []chan struct{}
	done      chan struct{}
}

func newServiceMonitor() *serviceMonitor {
	return &serviceMonitor{
		listeners: []chan struct{}{},
	}
}

func (m *serviceMonitor) start() {
	manager, err := mgr.Connect()
	if err != nil {
		log.Printf("E! [windows_event_log] Unable to connect to Windows service manager: %v", err)
		return
	}

	service, err := manager.OpenService(serviceName)
	if err != nil {
		log.Printf("E! [windows_event_log] Unable to observe Windows event log service: %v", err)
		return
	}

	ticker := time.NewTicker(serviceCheckInterval)
	defer ticker.Stop()

	// get initial service PID
	oldPID, _ := getPID(service)
	for {
		select {
		case <-ticker.C:
			newPID, err := getPID(service)
			if err == nil && oldPID != newPID {
				log.Printf("D! [windows_event_log] Detected Windows event log service restart")
				oldPID = newPID
				m.notify()
			}
		case <-m.done:
			return
		}
	}
}

func (m *serviceMonitor) stop() {
	close(m.done)
}

func (m *serviceMonitor) addListener(listener chan struct{}) {
	m.listeners = append(m.listeners, listener)
}

func (m *serviceMonitor) notify() {
	for _, l := range m.listeners {
		select {
		case l <- struct{}{}:
		default:
		}
	}
}

func getPID(service statusChecker) (uint32, error) {
	status, err := service.Query()
	if err != nil {
		return 0, err
	}
	if status.State == svc.Running {
		return status.ProcessId, nil
	}
	return 0, errServiceNotRunning
}
