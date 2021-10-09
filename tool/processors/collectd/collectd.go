// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectd

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/collectd"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/defaultConfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	if ctx.OsParameter == util.OsTypeWindows {
		return
	}
	yes := util.Yes("Do you want to monitor metrics from CollectD? WARNING: CollectD must be installed or the Agent will fail to start")
	if yes {
		collection := config.MetricsConf().Collection()
		collection.CollectD = new(collectd.CollectD)
		if collection.StatsD != nil {
			collection.CollectD.MetricsAggregationInterval = collection.StatsD.MetricsAggregationInterval
		} else {
			collection.CollectD.MetricsAggregationInterval = 60
		}
	}
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	if ctx.OsParameter == util.OsTypeWindows {
		return migration.Processor
	} else {
		return defaultConfig.Processor
	}
}
