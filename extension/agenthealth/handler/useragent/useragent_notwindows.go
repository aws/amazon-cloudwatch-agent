//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"github.com/influxdata/telegraf/models"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

func (ua *userAgent) detectWindowsEventLogFeatures(input *models.RunningInput, winFeatures collections.Set[string]) {
	// No-op on non-Windows platforms
}
