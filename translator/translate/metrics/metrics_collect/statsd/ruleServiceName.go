// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
)

type ServiceName struct {
}

const SectionkeyServicename = "service.name"

func (obj *ServiceName) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(SectionkeyServicename, "", input)
	returnKey = "service_name"

	if returnVal == "" {
		returnVal = metrics.GlobalMetricConfig.ServiceName
	}
	return
}

func init() {
	obj := new(ServiceName)
	RegisterRule(SectionkeyServicename, obj)
}
