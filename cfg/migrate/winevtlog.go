// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

func init() {
	AddRule(WindowsEventLogRule)
}

//[inputs]
//    [[inputs.windows_event_log]]
//+     destination = "cloudwatchlogs"
func WindowsEventLogRule(conf map[string]interface{}) error {
	ts := getItem(conf, "inputs", "windows_event_log")

	for _, t := range ts {
		_, ok := t["destination"]
		if !ok {
			t["destination"] = "cloudwatchlogs"
		}
	}

	return nil
}
