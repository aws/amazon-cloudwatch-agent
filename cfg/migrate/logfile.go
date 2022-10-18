// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

func init() {
	AddRule(LogFileRule)
}

//[inputs]
//    [[inputs.tail]]
//+     destination = "cloudwatchlogs"
//-     data_format = "value"
//-     data_type = "string"
//-     name_override = "raw_log_line"
func LogFileRule(conf map[string]interface{}) error {
	ts := getItem(conf, "inputs", "tail")

	if ts == nil {
		return nil
	}

	for _, t := range ts {
		delete(t, "data_format")
		delete(t, "data_type")
		delete(t, "name_override")
		_, ok := t["destination"]
		if !ok {
			t["destination"] = "cloudwatchlogs"
		}
	}

	inputs := conf["inputs"].(map[string]interface{})
	delete(inputs, "tail")
	inputs["logfile"] = ts

	return nil
}
