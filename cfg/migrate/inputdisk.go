// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

func init() {
	AddRule(InputDiskRule)
}

//[inputs]
//[[inputs.disk]]
//-      drop_device = false
//+      tagexclude = ["mode"]
func InputDiskRule(conf map[string]interface{}) error {
	disks := getItem(conf, "inputs", "disk")

	for _, disk := range disks {
		delete(disk, "drop_device")

		tagexclude, ok := disk["tagexclude"].([]interface{})
		var newTagEx []interface{}
		if ok {
			newTagEx = append(newTagEx, tagexclude...)
		}
		newTagEx = append(newTagEx, "mode")

		disk["tagexclude"] = newTagEx
	}

	return nil
}
