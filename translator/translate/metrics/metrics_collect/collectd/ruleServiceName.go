// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type ServiceName struct {
}

const SectionkeyServicename = "service.name"

func (obj *ServiceName) ApplyRule(input interface{}) (string, interface{}) {
	returnKey, returnVal := translator.DefaultCase(SectionkeyServicename, "", input)

	parentKeyVal := metrics.GlobalMetricConfig.ServiceName
	if returnVal != "" {
		return common.Tags, map[string]interface{}{returnKey: returnVal}
	} else if parentKeyVal != "" {
		return common.Tags, map[string]interface{}{returnKey: parentKeyVal}
	}
	return "", nil
}

func init() {
	obj := new(ServiceName)
	RegisterRule(SectionkeyServicename, obj)
}
