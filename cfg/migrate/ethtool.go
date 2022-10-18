// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

func init() {
	AddRule(EthtoolRule)
}

//  [inputs]
//    [[inputs.ethtool]]
//+     fieldpass = ["bw_in_allowance_exceeded", "bw_out_allowance_exceeded", "pps_allowance_exceeded", "conntrack_allowance_exceeded", "linklocal_allowance_exceeded"]
//      interface_include = ["eth0", "eth1"]
//-     metrics_include = ["bw_in_allowance_exceeded", "bw_out_allowance_exceeded", "pps_allowance_exceeded", "conntrack_allowance_exceeded", "linklocal_allowance_exceeded"]
//      [inputs.ethtool.tags]
//        metricPath = "metrics"
func EthtoolRule(conf map[string]interface{}) error {
	es := getItem(conf, "inputs", "ethtool")

	for _, e := range es {
		fields := e["metrics_include"]
		delete(e, "metrics_include")
		e["fieldpass"] = fields
	}

	return nil
}
