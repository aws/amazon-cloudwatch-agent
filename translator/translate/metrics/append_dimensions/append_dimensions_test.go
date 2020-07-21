// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package append_dimensions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendDimensions(t *testing.T) {
	e := new(appendDimensions)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{
      "append_dimensions": {
        "ImageId": "${aws:ImageId}",
        "InstanceId": "${aws:InstanceId}",
        "InstanceType": "${aws:InstanceType}",
        "AutoScalingGroupName": "${aws:AutoScalingGroupName}"
      }
    }`), &input)
	if err == nil {
		_, actual := e.ApplyRule(input)
		expected := map[string]interface{}{
			"ec2tagger": []interface{}{
				map[string]interface{}{
					"ec2_instance_tag_keys": []string{"aws:autoscaling:groupName"},
					"ec2_metadata_tags": []string{
						"ImageId", "InstanceId", "InstanceType",
					},
					"refresh_interval_seconds": "0s",
				},
			},
		}
		assert.Equal(t, expected, actual, "Expect to be equal")
	} else {
		panic(err)
	}
}
