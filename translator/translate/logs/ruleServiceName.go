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
	_, result := translator.DefaultCase("service.name", "", input)

	if result == "" {
		result = agent.Global_Config.ServiceName
	}
	// Set global log service name
	GlobalLogConfig.ServiceName = result.(string)

	return
}
