// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

type ServiceName struct {
}

func (f *ServiceName) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase("service.name", "", input)
	returnKey = "service_name"

	if returnVal == "" {
		returnVal = agent.Global_Config.ServiceName
	}
	// Set global log service name
	GlobalLogConfig.ServiceName = returnVal.(string)

	return
}
