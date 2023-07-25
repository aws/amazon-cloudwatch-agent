// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package registerrules

// Rules register themselves during import with their parent rules in a hierarchy up until the root translator object.
// Because of this, when rules need to be registered and merged, this package should be imported as a whole
import (
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/csm"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/globaltags"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/files"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/files/collect_list"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/windows_events"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/windows_events/collect_list"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/ecs"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/kubernetes"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/dockerlabel"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/serviceendpoint"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery/taskdefinition"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/drop_origin"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metric_decoration"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/collectd"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/cpu"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/customizedmetrics"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/disk"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/diskio"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/ethtool"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/gpu"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/mem"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/net"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/netstat"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/processes"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/procstat"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/statsd"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect/swap"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/rollup_dimensions"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/translate/traces"
)
