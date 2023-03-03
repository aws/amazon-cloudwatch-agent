package clean

import "time"

const (
	KeepDurationOneDay = -1 * time.Hour * 24
	KeepDurationSixtyDay = KeepDurationOneDay * time.Duration(60)
	KeepDurationTwentySixHours = KeepDurationOneDay +  time.Hour * 2
)