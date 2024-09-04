// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
)

type ServiceName struct {
}

func (f *ServiceName) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase("service.name", "", input)
	returnKey = "service_name"

	if returnVal == "" {
		returnVal = logs.GlobalLogConfig.ServiceName
	}

	return
}

func init() {
	f := new(ServiceName)
	r := []Rule{f}
	RegisterRule("service.name", r)
}
