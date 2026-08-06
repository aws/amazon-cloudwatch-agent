// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package extract

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestExtractInstanceName(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		cmdline      []string
		cmdlineErr   error
		wantInstance string
	}{
		"DefaultInstance/NoCmdlineFlag": {
			cmdline:      []string{"sqlservr"},
			wantInstance: "MSSQLSERVER",
		},
		"DefaultInstance/CmdlineError": {
			cmdlineErr:   assert.AnError,
			wantInstance: "MSSQLSERVER",
		},
		"NamedInstance/SeparateArg": {
			cmdline:      []string{"sqlservr", "-s", "YOURDBINSTANCE2"},
			wantInstance: "YOURDBINSTANCE2",
		},
		"NamedInstance/AttachedArg": {
			cmdline:      []string{"sqlservr", "-sYOURDBINSTANCE2"},
			wantInstance: "YOURDBINSTANCE2",
		},
		"NamedInstance/LowercaseFlag": {
			cmdline:      []string{"sqlservr", "-s", "myinstance"},
			wantInstance: "MYINSTANCE",
		},
		"NamedInstance/WithOtherFlags": {
			cmdline:      []string{"sqlservr", "-f", "-s", "INST2", "-T", "3608"},
			wantInstance: "INST2",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			if tt.cmdlineErr != nil {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, tt.cmdlineErr)
			} else {
				mp.On("CmdlineSliceWithContext", ctx).Return(tt.cmdline, nil)
			}

			got := extractInstanceName(ctx, mp)
			assert.Equal(t, tt.wantInstance, got)
			mp.AssertExpectations(t)
		})
	}
}
