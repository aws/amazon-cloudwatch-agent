// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filter

import "time"

type Config struct {
	Process ProcessConfig `json:"process"`
}

type ProcessConfig struct {
	// MinUptime is the minimum uptime for a process to be included in detection results.
	MinUptime time.Duration `json:"min_uptime"`
	// ExcludeNames is the list of names to exclude from detection results.
	ExcludeNames []string `json:"exclude_names"`
}
