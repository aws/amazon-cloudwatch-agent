// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config/metric/statsd"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/collectd"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

func TestProcessor_Process(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	conf := new(data.Config)

	testutil.Type(inputChan, "2")
	Processor.Process(ctx, conf)
	assert.Nil(t, conf.MetricsConfig)

	testutil.Type(inputChan, "", "", "", "")
	Processor.Process(ctx, conf)
	statsConf := conf.MetricsConf().Collection().StatsD
	assert.NotNil(t, statsConf)
	assert.Equal(t, ":8125", statsConf.ServiceAddress)
	assert.Equal(t, 10, statsConf.MetricsCollectionInterval)
	assert.Equal(t, 60, statsConf.MetricsAggregationInterval)
}

func TestProcessor_NextProcessor(t *testing.T) {
	ctx := new(runtime.Context)
	assert.Equal(t, collectd.Processor, Processor.NextProcessor(ctx, nil))
}

func TestWhichPort(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	conf := new(statsd.StatsD)

	testutil.Type(inputChan, "")
	whichPort(conf)
	assert.Equal(t, ":8125", conf.ServiceAddress)

	testutil.Type(inputChan, "12345")
	whichPort(conf)
	assert.Equal(t, ":12345", conf.ServiceAddress)
}

func TestWhichCollectionInterval(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	conf := new(statsd.StatsD)

	testutil.Type(inputChan, "")
	whichMetricsCollectionInterval(conf)
	assert.Equal(t, 10, conf.MetricsCollectionInterval)

	testutil.Type(inputChan, "2")
	whichMetricsCollectionInterval(conf)
	assert.Equal(t, 30, conf.MetricsCollectionInterval)

	testutil.Type(inputChan, "3")
	whichMetricsCollectionInterval(conf)
	assert.Equal(t, 60, conf.MetricsCollectionInterval)
}

func TestWhichAggregationInterval(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	conf := new(statsd.StatsD)

	testutil.Type(inputChan, "1")
	whichMetricsAggregationInterval(conf)
	assert.Equal(t, 0, conf.MetricsAggregationInterval)

	testutil.Type(inputChan, "2")
	whichMetricsAggregationInterval(conf)
	assert.Equal(t, 10, conf.MetricsAggregationInterval)

	testutil.Type(inputChan, "3")
	whichMetricsAggregationInterval(conf)
	assert.Equal(t, 30, conf.MetricsAggregationInterval)

	testutil.Type(inputChan, "")
	whichMetricsAggregationInterval(conf)
	assert.Equal(t, 60, conf.MetricsAggregationInterval)
}
