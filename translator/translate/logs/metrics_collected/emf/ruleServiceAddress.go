// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/context"
)

type ServiceAddress struct {
}

const SectionKeyServiceAddress = "service_address"

func (obj *ServiceAddress) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	defaultServiceAddress := "udp://127.0.0.1:25888"
	if context.CurrentContext().RunInContainer() {
		defaultServiceAddress = "udp://:25888"
	}
	returnKey, returnVal = translator.DefaultCase(SectionKeyServiceAddress, defaultServiceAddress, input)
	return
}

func init() {
	obj := new(ServiceAddress)
	RegisterRule(SectionKeyServiceAddress, obj)
}
