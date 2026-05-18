// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

func TestCollectD_HappyCase(t *testing.T) {
	_, actual := testutil.UnmarshalAndApplyRule(t, `{"collectd": {
		"service_address": "udp://127.0.0.1:123",
		"name_prefix": "collectd_prefix_",
		"collectd_auth_file": "/etc/collectd/_auth_file",
		"collectd_security_level": "none",
		"collectd_typesdb": ["/usr/share/collectd/types.db", "/custom_location/types.db"],
		"metrics_aggregation_interval": 30
	}}`, new(CollectD))

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
	_, actual := testutil.UnmarshalAndApplyRule(t, `{"collectd": {}}`, new(CollectD))

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
	cleanup := testutil.MockEC2Metadata(&util.Metadata{InstanceID: "i-1234567890abcdef0", InstanceType: "t3.medium"})
	defer cleanup()

	_, actual := testutil.UnmarshalAndApplyRule(t, `{"collectd": {
		"service_address": "udp://127.0.0.1:123",
		"append_dimensions": {
			"InstanceId": "${aws:InstanceId}",
			"CustomDimension": "CustomValue"
		}
	}}`, new(CollectD))

	testutil.AssertDimensionsEqual(t, actual, map[string]interface{}{
		"aws:AggregationInterval": "60s",
		"CustomDimension":         "CustomValue",
		"InstanceId":              "i-1234567890abcdef0",
	})
}

func TestCollectD_WithAppendDimensionsAndAggregationInterval(t *testing.T) {
	_, actual := testutil.UnmarshalAndApplyRule(t, `{"collectd": {
		"metrics_aggregation_interval": 30,
		"append_dimensions": {
			"Environment": "Production",
			"Team": "Infrastructure"
		}
	}}`, new(CollectD))

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
	cleanup := testutil.MockEC2Metadata(&util.Metadata{InstanceID: "i-1234567890abcdef0", InstanceType: "t3.large", ImageID: "ami-12345678"})
	defer cleanup()

	_, actual := testutil.UnmarshalAndApplyRule(t, `{"collectd": {
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
	}}`, new(CollectD))

	testutil.AssertDimensionsEqual(t, actual, map[string]interface{}{
		"aws:AggregationInterval": "30s",
		"CustomTag":               "MyValue",
		"InstanceId":              "i-1234567890abcdef0",
		"InstanceType":            "t3.large",
		"ImageId":                 "ami-12345678",
	})
}

func TestCollectD_EmptyAppendDimensions(t *testing.T) {
	_, actual := testutil.UnmarshalAndApplyRule(t, `{"collectd": {
		"append_dimensions": {}
	}}`, new(CollectD))

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
