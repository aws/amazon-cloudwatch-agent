// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Check the case when the input is in "disk":{//specific configuration}
func TestDiskSpecificConfig(t *testing.T) {
	d := new(Disk)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{"disk":{"metrics_collection_interval":"60"}}`), &input)
	if err == nil {
		actualReturnKey, _ := d.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey, "Expect to be equal")
	} else {
		panic(err)
	}
	//Check whether provide specific config
	var input1 interface{}
	err = json.Unmarshal([]byte(`{"disk":{
					"resources": [
						"/", "/dev", "/sys"
					],
                    "ignore_file_system_types": [
                        "sysfs", "devtmpfs"
                    ],
					"measurement": [
						"free",
						"total",
						"used"
					]}}`), &input1)
	if err == nil {
		_, actualVal := d.ApplyRule(input1)
		expectedVal := []interface{}{map[string]interface{}{
			"ignore_fs":    []interface{}{"sysfs", "devtmpfs"},
			"mount_points": []interface{}{"/", "/dev", "/sys"},
			"fieldpass":    []string{"free", "total", "used"},
			"tagexclude":   []string{"mode"},
		},
		}
		assert.Equal(t, expectedVal, actualVal, "Expect to be equal")
	} else {
		panic(err)
	}

	//check when "drop_device" = true
	var input2 interface{}
	err = json.Unmarshal([]byte(`{"disk":{
					"resources": [
						"/", "/dev", "/sys"
					],
                    "ignore_file_system_types": [
                        "sysfs", "devtmpfs"
                    ],
					"measurement": [
						"free",
						"total",
						"used"
					],
					"drop_device": true
					}}`), &input2)
	if err == nil {
		_, actualValue := d.ApplyRule(input2)
		expectedValue := []interface{}{map[string]interface{}{
			"ignore_fs":    []interface{}{"sysfs", "devtmpfs"},
			"mount_points": []interface{}{"/", "/dev", "/sys"},
			"fieldpass":    []string{"free", "total", "used"},
			"tagexclude":   []string{"device", "mode"},
		},
		}
		assert.Equal(t, expectedValue, actualValue, "Expect to be equal")
	} else {
		panic(err)
	}

	//check when "drop_device" = false
	var input3 interface{}
	err = json.Unmarshal([]byte(`{"disk":{
					"resources": [
						"/", "/dev", "/sys"
					],
                    "ignore_file_system_types": [
                        "sysfs", "devtmpfs"
                    ],
					"measurement": [
						"free",
						"total",
						"used"
					],
					"drop_device": false
					}}`), &input3)
	if err == nil {
		_, actualValue := d.ApplyRule(input3)
		expectedValue := []interface{}{map[string]interface{}{
			"ignore_fs":    []interface{}{"sysfs", "devtmpfs"},
			"mount_points": []interface{}{"/", "/dev", "/sys"},
			"fieldpass":    []string{"free", "total", "used"},
			"tagexclude":   []string{"mode"},
		},
		}
		assert.Equal(t, expectedValue, actualValue, "Expect to be equal")
	} else {
		panic(err)
	}

}
