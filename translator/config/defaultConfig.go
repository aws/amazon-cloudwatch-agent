package config

var defaultLinuxOnPremConfig string = `
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

var defaultWindowsOnPremConfig string = `
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

var defaultLinuxEC2Config string = `
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

var defaultWindowsEC2Config string = `
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

var defaultLinuxECSNodeMetricConfig string = `
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
	if os == OS_TYPE_WINDOWS {
		if mode == ModeEC2 {
			return defaultWindowsEC2Config
		} else {
			return defaultWindowsOnPremConfig
		}
	} else {
		if mode == ModeEC2 {
			return defaultLinuxEC2Config
		} else {
			return defaultLinuxOnPremConfig
		}
	}
}
