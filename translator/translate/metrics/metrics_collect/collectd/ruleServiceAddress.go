// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type ServiceAddress struct {
}

const SectionKey_ServiceAddress = "service_address"

func (obj *ServiceAddress) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_ServiceAddress, "udp://127.0.0.1:25826", input)
	return
}

func init() {
	obj := new(ServiceAddress)
	RegisterRule(SectionKey_ServiceAddress, obj)
}
