//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"github.com/influxdata/telegraf/models"
)

func (ua *userAgent) setWindowsEventLogFeatureFlags(input *models.RunningInput) {
	// No-op on non-Windows platforms
}
