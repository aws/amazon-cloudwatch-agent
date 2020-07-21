// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

func InitWindowsObject(objectName, instances, counters, measurement string) map[string]interface{} {
	res := map[string]interface{}{
		"ObjectName":  objectName,
		"Instances":   []string{instances},
		"Measurement": measurement,
		"Counters":    []string{counters},
	}
	return res
}
