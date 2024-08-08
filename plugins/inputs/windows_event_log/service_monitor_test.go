// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package windows_event_log

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/svc"
)

type mockStatusCheck struct {
	status svc.Status
	err    error
}

func (m *mockStatusCheck) Query() (svc.Status, error) {
	return m.status, m.err
}

func TestGetPID(t *testing.T) {
	testErr := errors.New("test error")
	testCases := map[string]struct {
		status  svc.Status
		err     error
		wantPID uint32
		wantErr error
	}{
		"WithQueryError": {
			err:     testErr,
			wantPID: 0,
			wantErr: testErr,
		},
		"WithStoppedService": {
			status: svc.Status{
				State:     svc.Stopped,
				ProcessId: 0,
			},
			wantPID: 0,
			wantErr: errServiceNotRunning,
		},
		"WithRunningService": {
			status: svc.Status{
				State:     svc.Running,
				ProcessId: 123,
			},
			wantPID: 123,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			gotPID, gotErr := getPID(&mockStatusCheck{status: testCase.status, err: testCase.err})
			assert.Equal(t, testCase.wantPID, gotPID)
			assert.Equal(t, testCase.wantErr, gotErr)
		})
	}
}
