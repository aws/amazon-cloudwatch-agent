// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import _ "embed"

// Platform-specific default configs. Unlike otel.json/otel_windows.json (which
// are OS-build-tagged), these have no OS constraint - K8s/ECS are Linux - so
// they live here rather than in a build-tagged file.
//
//go:embed defaults/otel_ecs.json
var defaultOtelECSConfig string

//go:embed defaults/otel_k8s.json
var defaultOtelK8sConfig string

// otelConfigName is the shared map key for the otel default config and its
// platform variants (e.g. -c default:otel).
const otelConfigName = "otel"

// defaultConfigs holds the base configs addressable by name.
var defaultConfigs = map[string]string{
	otelConfigName: defaultOtelConfig,
}

// platform enumerates the environments that get a platform-specific default
// config variant. VMs (EC2, Azure VM) use the base config; K8s and ECS get
// their own variants since host-level scraping inside a container reports the
// container, not the host.
type platform int

const (
	platformK8s platform = iota
	platformECS
)

// platformConfigs holds the platform variants. Kept out of defaultConfigs so
// they are NOT addressable by name (e.g. -c default:otel_ecs on a VM).
var platformConfigs = map[platform]map[string]string{
	platformK8s: {otelConfigName: defaultOtelK8sConfig},
	platformECS: {otelConfigName: defaultOtelECSConfig},
}

// DefaultJSONConfigFor returns the platform-specific variant when the caller's
// detected platform has one, falling back to the named base config otherwise.
// Kubernetes takes precedence over ECS.
func DefaultJSONConfigFor(name string, isKubernetes, isECS bool) (string, bool) {
	switch {
	case isKubernetes:
		if cfg, ok := platformConfigs[platformK8s][name]; ok {
			return cfg, true
		}
	case isECS:
		if cfg, ok := platformConfigs[platformECS][name]; ok {
			return cfg, true
		}
	}
	cfg, ok := defaultConfigs[name]
	return cfg, ok
}

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
