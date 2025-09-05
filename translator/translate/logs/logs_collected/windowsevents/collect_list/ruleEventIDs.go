// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const EventIDsSectionKey = "event_ids"

type EventIDs struct {
}

func isValidEventID(eventID int) bool {

	// Reference: https://learn.microsoft.com/en-us/windows/win32/winauto/allocation-of-winevent-ids
	// Documentation for Windows Event Log identifiers and ranges
	const (
		minEventID = 0
		maxEventID = 65535
	)
	return eventID >= minEventID && eventID <= maxEventID
}

func (f *EventIDs) ApplyRule(input interface{}) (string, interface{}) {
	m, ok := input.(map[string]interface{})
	if !ok {
		return "", nil
	}

	if _, exists := m[EventIDsSectionKey]; exists {
		_, eventIDs := translator.DefaultIntegralArrayCase(EventIDsSectionKey, []interface{}{}, input)

		// Validate each event ID
		if eventIDsArray, ok := eventIDs.([]int); ok {
			for i, eventID := range eventIDsArray {
				if !isValidEventID(eventID) {
					translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("Invalid event ID %d at index %d. Event IDs must be between 0 and 65535.", eventID, i))
				}
			}
		}
		return EventIDsSectionKey, eventIDs
	}
	return EventIDsSectionKey, nil
}

func init() {
	e := new(EventIDs)
	RegisterRule(EventIDsSectionKey, e)
}
