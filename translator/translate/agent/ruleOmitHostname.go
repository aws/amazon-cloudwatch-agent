// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"os"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

type OmitHostname struct {
}

func (o *OmitHostname) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	if os.Getenv(config.RUN_IN_CONTAINER) == config.RUN_IN_CONTAINER_TRUE {
		returnKey, returnVal = translator.DefaultCase("omit_hostname", true, input)
	} else {
		returnKey, returnVal = translator.DefaultCase("omit_hostname", false, input)
	}
	return
}

func init() {
	o := new(OmitHostname)
	RegisterRule("omit_hostname", o)
}
