// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package plugins

import (
	//Enable cloudwatch-agent process plugins
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/processors/delta"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/processors/ec2tagger"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/processors/ecsdecorator"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/processors/emfProcessor"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/processors/k8sdecorator"

	// Enabled parsers registry
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/parsers"

	// Enabled cloudwatch-agent input plugins
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/awscsm"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/cadvisor"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/demo"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/k8sapiserver"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/logfile"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/prometheus_scraper"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/statsd"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/win_perf_counters"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/inputs/windows_event_log"

	// Enabled cloudwatch-agent output plugins
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/awscsm"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/cloudwatch"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/cloudwatchlogs"
	_ "github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/console"

	// Enabled telegraf input plugins
	// NOTE: any plugins that are dependencies of the plugins enabled will be enabled too
	// e.g.: cpu plguin from telegraf would enable the system plugin as its dependency
	_ "github.com/influxdata/telegraf/plugins/inputs/cpu"
	_ "github.com/influxdata/telegraf/plugins/inputs/disk"
	_ "github.com/influxdata/telegraf/plugins/inputs/diskio"
	_ "github.com/influxdata/telegraf/plugins/inputs/ethtool"
	_ "github.com/influxdata/telegraf/plugins/inputs/mem"
	_ "github.com/influxdata/telegraf/plugins/inputs/net"
	_ "github.com/influxdata/telegraf/plugins/inputs/nvidia_smi"
	_ "github.com/influxdata/telegraf/plugins/inputs/processes"
	_ "github.com/influxdata/telegraf/plugins/inputs/procstat"
	_ "github.com/influxdata/telegraf/plugins/inputs/socket_listener"
	_ "github.com/influxdata/telegraf/plugins/inputs/swap"
)
