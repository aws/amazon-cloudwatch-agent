// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

func TestCollectD_HappyCase(t *testing.T) {
	obj := new(CollectD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"collectd": {
		"service_address": "udp://127.0.0.1:123",
		"name_prefix": "collectd_prefix_",
		"collectd_auth_file": "/etc/collectd/_auth_file",
		"collectd_security_level": "none",
		"collectd_typesdb": ["/usr/share/collectd/types.db", "/custom_location/types.db"],
		"metrics_aggregation_interval": 30
	}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"data_format":             "collectd",
			"service_address":         "udp://127.0.0.1:123",
			"name_prefix":             "collectd_prefix_",
			"collectd_auth_file":      "/etc/collectd/_auth_file",
			"collectd_security_level": "none",
			"collectd_typesdb":        []interface{}{"/usr/share/collectd/types.db", "/custom_location/types.db"},
			"tags":                    map[string]interface{}{"aws:AggregationInterval": "30s"},
		},
	}

	assert.Equal(t, expect, actual)
}

func TestCollectD_MinimumConfig(t *testing.T) {
	obj := new(CollectD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"collectd": {}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"data_format":             "collectd",
			"service_address":         "udp://127.0.0.1:25826",
			"name_prefix":             "collectd_",
			"collectd_auth_file":      "/etc/collectd/auth_file",
			"collectd_security_level": "encrypt",
			"collectd_typesdb":        []interface{}{"/usr/share/collectd/types.db"},
			"tags":                    map[string]interface{}{"aws:AggregationInterval": "60s"},
		},
	}

	assert.Equal(t, expect, actual)
}

func TestCollectD_WithAppendDimensions(t *testing.T) {
	// Mock EC2 metadata to avoid 6s IMDS timeout
	originalProvider := util.Ec2MetadataInfoProvider
	util.Ec2MetadataInfoProvider = func() *util.Metadata {
		return &util.Metadata{InstanceID: "i-1234567890abcdef0", InstanceType: "t3.medium"}
	}
	defer func() { util.Ec2MetadataInfoProvider = originalProvider }()

	obj := new(CollectD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"collectd": {
		"service_address": "udp://127.0.0.1:123",
		"append_dimensions": {
			"InstanceId": "${aws:InstanceId}",
			"CustomDimension": "CustomValue"
		}
	}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	actualMap := actual.([]interface{})[0].(map[string]interface{})
	tags := actualMap["tags"].(map[string]interface{})

	assert.Equal(t, "60s", tags["aws:AggregationInterval"])
	assert.Equal(t, "CustomValue", tags["CustomDimension"])
	assert.Equal(t, "i-1234567890abcdef0", tags["InstanceId"])
}

func TestCollectD_WithAppendDimensionsAndAggregationInterval(t *testing.T) {
	obj := new(CollectD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"collectd": {
		"metrics_aggregation_interval": 30,
		"append_dimensions": {
			"Environment": "Production",
			"Team": "Infrastructure"
		}
	}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"data_format":             "collectd",
			"service_address":         "udp://127.0.0.1:25826",
			"name_prefix":             "collectd_",
			"collectd_auth_file":      "/etc/collectd/auth_file",
			"collectd_security_level": "encrypt",
			"collectd_typesdb":        []interface{}{"/usr/share/collectd/types.db"},
			"tags": map[string]interface{}{
				"aws:AggregationInterval": "30s",
				"Environment":             "Production",
				"Team":                    "Infrastructure",
			},
		},
	}

	assert.Equal(t, expect, actual)
}

func TestCollectD_WithFullConfigAndAppendDimensions(t *testing.T) {
	// Mock EC2 metadata to avoid IMDS timeout
	originalProvider := util.Ec2MetadataInfoProvider
	util.Ec2MetadataInfoProvider = func() *util.Metadata {
		return &util.Metadata{InstanceID: "i-1234567890abcdef0", InstanceType: "t3.large", ImageID: "ami-12345678"}
	}
	defer func() { util.Ec2MetadataInfoProvider = originalProvider }()

	obj := new(CollectD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"collectd": {
		"service_address": "udp://127.0.0.1:123",
		"name_prefix": "collectd_prefix_",
		"collectd_auth_file": "/etc/collectd/_auth_file",
		"collectd_security_level": "none",
		"collectd_typesdb": ["/usr/share/collectd/types.db", "/custom_location/types.db"],
		"metrics_aggregation_interval": 30,
		"append_dimensions": {
			"InstanceId": "${aws:InstanceId}",
			"InstanceType": "${aws:InstanceType}",
			"ImageId": "${aws:ImageId}",
			"CustomTag": "MyValue"
		}
	}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	actualMap := actual.([]interface{})[0].(map[string]interface{})
	tags := actualMap["tags"].(map[string]interface{})

	assert.Equal(t, "30s", tags["aws:AggregationInterval"])
	assert.Equal(t, "MyValue", tags["CustomTag"])
	assert.Equal(t, "i-1234567890abcdef0", tags["InstanceId"])
	assert.Equal(t, "t3.large", tags["InstanceType"])
	assert.Equal(t, "ami-12345678", tags["ImageId"])
}

func TestCollectD_EmptyAppendDimensions(t *testing.T) {
	obj := new(CollectD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"collectd": {
		"append_dimensions": {}
	}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"data_format":             "collectd",
			"service_address":         "udp://127.0.0.1:25826",
			"name_prefix":             "collectd_",
			"collectd_auth_file":      "/etc/collectd/auth_file",
			"collectd_security_level": "encrypt",
			"collectd_typesdb":        []interface{}{"/usr/share/collectd/types.db"},
			"tags": map[string]interface{}{
				"aws:AggregationInterval": "60s",
			},
		},
	}

	assert.Equal(t, expect, actual)
}
