// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	EventFormatSectionKey = "event_format"

	EventFormatXML       = "xml"  //xml format in windows event viewer
	EVentFormatPlainText = "text" //old ssm agent format
)

type EventFormat struct {
}

func (r *EventFormat) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(EventFormatSectionKey, "", input)
	if returnVal == "" {
		return
	}
	if returnVal != EventFormatXML && returnVal != EVentFormatPlainText {
		translator.AddErrorMessages(GetCurPath()+EventFormatSectionKey, fmt.Sprintf("event_format value %s is not a valid value.", returnVal))
		return
	}
	returnKey = EventFormatSectionKey
	return
}

func init() {
	r := new(EventFormat)
	RegisterRule(EventFormatSectionKey, r)
}
