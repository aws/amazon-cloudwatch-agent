// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

func init() {
	AddRule(DiskIORule)
}

//  [inputs]
//    [[inputs.diskio]]
//      fieldpass = ["io_time", "write_bytes", "read_bytes", "writes", "reads"]
//-     report_deltas = true
//      [inputs.diskio.tags]
//        metricPath = "metrics"
//+       report_deltas = "true"
//
//  [processors]
//+   [[processors.delta]]
func DiskIORule(conf map[string]interface{}) error {
	ds := getItem(conf, "inputs", "diskio")
	if ds == nil {
		return nil
	}

	for _, d := range ds {
		delete(d, "report_deltas")

		ts, ok := d["tags"].(map[string]interface{})
		if ok {
			ts["report_deltas"] = "true"
		} else {
			d["tags"] = map[string]interface{}{"report_deltas": "true"}
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
