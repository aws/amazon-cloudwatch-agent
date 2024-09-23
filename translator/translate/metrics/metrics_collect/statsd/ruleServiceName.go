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

func (obj *ServiceName) ApplyRule(input interface{}) (string, interface{}) {
	_, returnVal := translator.DefaultCase(SectionkeyServicename, "", input)
	returnKey := "service.name"

	if returnVal == "" {
		returnVal = metrics.GlobalMetricConfig.ServiceName
	}
	return "tags", map[string]interface{}{returnKey: returnVal}
}

func init() {
	obj := new(ServiceName)
	RegisterRule(SectionkeyServicename, obj)
}
