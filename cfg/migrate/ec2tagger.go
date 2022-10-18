// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

func init() {
	AddRule(Ec2TaggerRule)
}

//[processors]
//[[processors.ec2tagger]]
//*       refresh_interval_seconds = "0s" // changed from 'refresh_interval_seconds = "2147483647s"'
func Ec2TaggerRule(conf map[string]interface{}) error {
	taggers := getItem(conf, "processors", "ec2tagger")

	for _, tagger := range taggers {
		ris, ok := tagger["refresh_interval_seconds"].(string)
		if !ok {
			continue
		}
		if ris == "2147483647s" {
			tagger["refresh_interval_seconds"] = "0s"
		}
	}

	return nil
}
