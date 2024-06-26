// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type ServiceName struct {
}

func (f *ServiceName) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, result := translator.DefaultCase("service.name", "", input)

	// Set global agent service name
	Global_Config.ServiceName = result.(string)
	return
}

func init() {
	f := new(ServiceName)
	RegisterRule("service.name", f)
}
