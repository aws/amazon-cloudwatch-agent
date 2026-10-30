// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dpdk

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

// The dpdk plugin collects device statistics (including the ENA extended
// statistics such as pps_allowance_exceeded, bw_in_allowance_exceeded, ...)
// from applications built with DPDK via the v2 telemetry socket. Interfaces
// bound to DPDK are detached from the kernel driver, so the ethtool plugin
// cannot see them; this plugin covers that gap.
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/monitoring-network-performance-ena.html#network-performance-metrics-dpdk
//
//	"dpdk" : {
//	    "socket_path": "/var/run/dpdk/rte/dpdk_telemetry.v2",
//	    "device_types": ["ethdev"],
//	    "additional_commands": [],
//	    "ethdev_exclude_commands": ["/ethdev/link_status"],
//	    "metrics_include": [
//	        "pps_allowance_exceeded",
//	        "bw_in_allowance_exceeded"
//	    ],
//	    "append_dimensions":{
//		key:value
//	     }
//
// }

const SectionKey_Dpdk = "dpdk"

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Dpdk struct {
}

func (n *Dpdk) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArr := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Dpdk]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If exists, process it
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Dpdk], ChildRule, result)
		resArr = append(resArr, result)
		returnKey = SectionKey_Dpdk
		returnVal = resArr
		//Process tags
		util.ProcessAppendDimensions(m[SectionKey_Dpdk].(map[string]interface{}), SectionKey_Dpdk, result)
	}

	return
}

func init() {
	n := new(Dpdk)
	parent.RegisterLinuxRule(SectionKey_Dpdk, n)
}
