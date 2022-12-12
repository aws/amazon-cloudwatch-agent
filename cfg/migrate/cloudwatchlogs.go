// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

func init() {
	AddRule(CloudWatchLogsRule)
}

//[outputs]
//    [[outputs.cloudwatchlogs]]
//-     destination = "cloudwatchlogs"
func CloudWatchLogsRule(conf map[string]interface{}) error {
	cs := getItem(conf, "outputs", "cloudwatchlogs")

	for _, c := range cs {
		delete(c, "file_state_folder")
	}

	return nil
}
