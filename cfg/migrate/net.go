// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

func init() {
	AddRule(NetRule)
}

//    [[inputs.net]]
//      fieldpass = ["bytes_sent", "bytes_recv", "drop_in", "drop_out"]
//      interfaces = ["eth0"]
//-     report_deltas = true
//      [inputs.net.tags]
//        "aws:StorageResolution" = "true"
//        metricPath = "metrics"
//+       report_deltas = "true"
//
//  [processors]
//+   [[processors.delta]]
func NetRule(conf map[string]interface{}) error {
	ns := getItem(conf, "inputs", "net")
	if ns == nil {
		return nil
	}

	for _, n := range ns {
		delete(n, "report_deltas")

		ts, ok := n["tags"].(map[string]interface{})
		if ok {
			ts["report_deltas"] = "true"
		} else {
			n["tags"] = map[string]interface{}{"report_deltas": "true"}
		}
	}

	ps, ok := conf["processors"].(map[string]interface{})
	if !ok {
		ps = make(map[string]interface{})
		conf["processors"] = ps
	}

	empty := make(map[string]interface{})
	ps["delta"] = []map[string]interface{}{empty}

	return nil
}
