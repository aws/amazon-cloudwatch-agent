// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/csm"
)

type DataFormat struct {
}

func (obj *DataFormat) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	csm_listener := map[string]interface{}{}
	csm_listener[csm.DataFormatKey] = "aws_csm"

	returnKey = ConfInputPluginKey
	returnVal = csm_listener
	return
}

func init() {
	obj := new(DataFormat)
	RegisterRule(csm.DataFormatKey, obj)
}
