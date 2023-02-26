// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/internal/csm"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

type Port struct {
}

func applyServiceAddressesRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, inputAddresses := translator.DefaultStringArrayCase(csm.ServiceAddressesKey, []string{}, input)
	stringAddresses := inputAddresses.([]string)
	addresses := []string{}

	metadata := util.GetMetadataInfo(util.Ec2MetadataInfoProvider)

	for _, addr := range stringAddresses {
		resolvedAddr := util.ResolvePlaceholder(addr, metadata)
		if checkCsmSocketListener(resolvedAddr) {
			addresses = append(addresses, resolvedAddr)
		}
	}

	if len(addresses) < 1 {
		translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("Client-side monitoring requires at least one value for service_addresses."))
		return "", nil
	}

	csm_listener := map[string]interface{}{}
	csm_listener[ConfInputAddressKey] = addresses

	returnKey = ConfInputPluginKey
	returnVal = csm_listener

	return
}

func applyPortRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, val := translator.DefaultIntegralCase(csm.PortKey, float64(csm.DefaultPort), input)

	addresses := []string{}

	ipv4addr := computeIPv4LoopbackAddressFromPort(val)
	if checkCsmSocketListener(ipv4addr) {
		addresses = append(addresses, ipv4addr)
	}

	ipv6addr := computeIPv6LoopbackAddressFromPort(val)
	if checkCsmSocketListener(ipv6addr) {
		addresses = append(addresses, ipv6addr)
	}

	if len(addresses) == 0 {
		translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("Client-side monitoring unable to successfully bind to at least one listener socket on port %v", val))
		return "", nil
	}

	csm_listener := map[string]interface{}{}
	csm_listener[ConfInputAddressKey] = addresses

	returnKey = ConfInputPluginKey
	returnVal = csm_listener

	return
}

func (p *Port) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	inputMap := input.(map[string]interface{})
	_, portExists := inputMap[csm.PortKey]
	_, serviceAddressesExists := inputMap[csm.ServiceAddressesKey]

	if portExists && serviceAddressesExists {
		translator.AddErrorMessages("", "Both port and service_addresses cannot be present in CSM configuration.")
		return "", nil
	}

	if serviceAddressesExists {
		returnKey, returnVal = applyServiceAddressesRule(input)
	} else {
		returnKey, returnVal = applyPortRule(input)
	}

	return
}

func init() {
	p := new(Port)
	RegisterRule(csm.PortKey, p)
}
