// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type ServiceAddress struct {
}

const SectionKey_ServiceAddress = "service_address"

func (obj *ServiceAddress) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_ServiceAddress, ":8125", input)
	return
}

func init() {
	obj := new(ServiceAddress)
	RegisterRule(SectionKey_ServiceAddress, obj)
}
