// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package wineventlog

import (
	"fmt"
	"time"
)

const (
	bookmarkTemplate         = `<BookmarkList><Bookmark Channel="%s" RecordId="%d" IsCurrent="True"/></BookmarkList>`
	eventLogQueryTemplate    = `<QueryList><Query Id="0"><Select Path="%s">%s</Select></Query></QueryList>`
	eventLogLevelFilter      = "Level='%s'"
	eventLogeventIDFilter    = "EventID='%d'"
	eventIgnoreOldFilter     = "TimeCreated[timediff(@SystemTime) &lt;= %d]"
	emptySpaceScanLength     = 100
	UnknownBytesPerCharacter = 0

	CRITICAL    = "CRITICAL"
	ERROR       = "ERROR"
	WARNING     = "WARNING"
	INFORMATION = "INFORMATION"
	VERBOSE     = "VERBOSE"
	UNKNOWN     = "UNKNOWN"
)

func createFilterQuery(levels []string, eventIDs []int) string {
	var filterLevels string
	for _, level := range levels {
		if filterLevels == "" {
			filterLevels = fmt.Sprintf(eventLogLevelFilter, level)
		} else {
			filterLevels = filterLevels + " or " + fmt.Sprintf(eventLogLevelFilter, level)
		}
	}

	//EventID filtering
	var filterEventID string
	for i, eventID := range eventIDs {
		if i == 0 {
			filterEventID = fmt.Sprintf(eventLogeventIDFilter, eventID)
		} else {
			filterEventID = filterEventID + " or " + fmt.Sprintf(eventLogeventIDFilter, eventID)
		}
	}

	//query results
	var query string
	if filterLevels != "" && filterEventID != "" {
		query = filterLevels + " and " + filterEventID
	} else if filterLevels != "" && filterEventID == "" {
		query = filterLevels
	} else if filterLevels == "" && filterEventID != "" {
		query = filterEventID
	}

	//Ignore events older than 2 weeks
	cutOffPeriod := (time.Hour * 24 * 14).Nanoseconds()
	ignoreOlderThanTwoWeeksFilter := fmt.Sprintf(eventIgnoreOldFilter, cutOffPeriod/int64(time.Millisecond))
	if filterLevels != "" && filterEventID != "" {
		query = "*[System[(" + query + ") and " + ignoreOlderThanTwoWeeksFilter + "]]"
	} else if filterLevels != "" {
		query = "*[System[(" + filterLevels + ") and " + ignoreOlderThanTwoWeeksFilter + "]]"
	} else if filterEventID != "" {
		query = "*[System[(" + filterEventID + ") and " + ignoreOlderThanTwoWeeksFilter + "]]"
	} else {
		query = "*[System[" + ignoreOlderThanTwoWeeksFilter + "]]"
	}

	return query
}
