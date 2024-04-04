// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package clean

import "time"

const (
	KeepDurationOneWeek        = KeepDurationOneDay * 7
	KeepDurationFourDays       = KeepDurationOneDay * 4
	KeepDurationOneDay         = -1 * time.Hour * 24
	KeepDurationSixtyDay       = KeepDurationOneDay * time.Duration(60)
	KeepDurationTwentySixHours = KeepDurationOneDay + time.Hour*2
)
