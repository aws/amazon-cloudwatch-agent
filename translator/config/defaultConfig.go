// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

var defaultLinuxOnPremConfig = `
{
	"agent": {
		"run_as_user": "cwagent"
	},
	"metrics": {
		"metrics_collected": {
			"mem": {
				"measurement": [
					"mem_used_percent"
				]
			},
			"disk": {
				"measurement": [
					"used_percent"
				],
				"resources": [
					"*"
				]
			}
		}
	}
}
`

var defaultDarwinOnPremConfig = `
{
	"agent": {
		"run_as_user": "cwagent"
	},
	"metrics": {
		"metrics_collected": {
			"mem": {
				"measurement": [
					"mem_used_percent"
				]
			},
			"disk": {
				"measurement": [
					"used_percent"
				],
				"resources": [
					"*"
				]
			}
		}
	}
}
`

var defaultWindowsOnPremConfig = `
{
	"metrics": {
		"metrics_collected": {
			"Memory": {
				"measurement": [
					"% Committed Bytes In Use"
				]
			},
			"LogicalDisk": {
				"measurement": [
					"% Free Space"
				],
				"resources": [
					"*"
				]
			}
		}
	}
}
`

var defaultLinuxEC2Config = `
{
	"agent": {
		"run_as_user": "cwagent"
	},
	"metrics": {
		"metrics_collected": {
			"mem": {
				"measurement": [
					"mem_used_percent"
				]
			},
			"disk": {
				"measurement": [
					"used_percent"
				],
				"resources": [
					"*"
				]
			}
		},
		"append_dimensions": {
			"ImageId": "${aws:ImageId}",
			"InstanceId": "${aws:InstanceId}",
			"InstanceType": "${aws:InstanceType}",
			"AutoScalingGroupName": "${aws:AutoScalingGroupName}"
		}
	}
}
`

var defaultDarwinEC2Config = `
{
	"agent": {
		"run_as_user": "cwagent"
	},
	"metrics": {
		"metrics_collected": {
			"mem": {
				"measurement": [
					"mem_used_percent"
				]
			},
			"disk": {
				"measurement": [
					"used_percent"
				],
				"resources": [
					"*"
				]
			}
		},
		"append_dimensions": {
			"ImageId": "${aws:ImageId}",
			"InstanceId": "${aws:InstanceId}",
			"InstanceType": "${aws:InstanceType}",
			"AutoScalingGroupName": "${aws:AutoScalingGroupName}"
		}
	}
}
`

var defaultWindowsEC2Config = `
{
	"metrics": {
		"metrics_collected": {
			"Memory": {
				"measurement": [
					"% Committed Bytes In Use"
				]
			},
			"LogicalDisk": {
				"measurement": [
					"% Free Space"
				],
				"resources": [
					"*"
				]
			}
		},
		"append_dimensions": {
			"ImageId": "${aws:ImageId}",
			"InstanceId": "${aws:InstanceId}",
			"InstanceType": "${aws:InstanceType}",
			"AutoScalingGroupName": "${aws:AutoScalingGroupName}"
		}
	}
}
`

var defaultLinuxECSNodeMetricConfig = `
{
  "logs": {
    "metrics_collected": {
        "ecs": {}
    }
  }
}
`

func DefaultECSJsonConfig() string {
	return defaultLinuxECSNodeMetricConfig
}
func DefaultJsonConfig(os string, mode string) string {
	switch os {
	case OS_TYPE_WINDOWS:
		if mode == ModeEC2 {
			return defaultWindowsEC2Config
		} else {
			return defaultWindowsOnPremConfig
		}
	case OS_TYPE_DARWIN:
		if mode == ModeEC2 {
			return defaultDarwinEC2Config
		} else {
			return defaultDarwinOnPremConfig
		}
	default:
		if mode == ModeEC2 {
			return defaultLinuxEC2Config
		} else {
			return defaultLinuxOnPremConfig
		}
	}
}
