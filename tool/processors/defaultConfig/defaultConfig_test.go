// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultConfig

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration/linux"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/question"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

func TestProcessor_Process(t *testing.T) {

	ctx := new(runtime.Context)
	conf := new(data.Config)

	Processor.Process(ctx, conf)
	assert.Equal(t, new(runtime.Context), ctx)
	assert.Equal(t, new(data.Config), conf)
}

// basic metrics config
var basicMetricsConf = map[string]interface{}{
	"metrics": map[string]interface{}{
		"aggregation_dimensions": [][]string{{"InstanceId"}},
		"append_dimensions":      map[string]interface{}{"AutoScalingGroupName": "${aws:AutoScalingGroupName}", "ImageId": "${aws:ImageId}", "InstanceId": "${aws:InstanceId}", "InstanceType": "${aws:InstanceType}"},
		"metrics_collected": map[string]interface{}{
			"disk": map[string]interface{}{"resources": []string{"*"}, "metrics_collection_interval": 60, "measurement": []string{"used_percent"}},
			"mem":  map[string]interface{}{"metrics_collection_interval": 60, "measurement": []string{"mem_used_percent"}}}}}

// standard metrics config
var standardMetricsConf = map[string]interface{}{
	"metrics": map[string]interface{}{
		"aggregation_dimensions": [][]string{{"InstanceId"}},
		"append_dimensions":      map[string]interface{}{"ImageId": "${aws:ImageId}", "InstanceId": "${aws:InstanceId}", "InstanceType": "${aws:InstanceType}", "AutoScalingGroupName": "${aws:AutoScalingGroupName}"},
		"metrics_collected": map[string]interface{}{
			"cpu":    map[string]interface{}{"resources": []string{"*"}, "totalcpu": false, "metrics_collection_interval": 60, "measurement": []string{"cpu_usage_idle", "cpu_usage_iowait", "cpu_usage_user", "cpu_usage_system"}},
			"disk":   map[string]interface{}{"resources": []string{"*"}, "metrics_collection_interval": 60, "measurement": []string{"used_percent", "inodes_free"}},
			"diskio": map[string]interface{}{"resources": []string{"*"}, "metrics_collection_interval": 60, "measurement": []string{"io_time"}},
			"mem":    map[string]interface{}{"metrics_collection_interval": 60, "measurement": []string{"mem_used_percent"}},
			"swap":   map[string]interface{}{"metrics_collection_interval": 60, "measurement": []string{"swap_used_percent"}}}}}

// advanced metrics config
var advancedMetricsConf = map[string]interface{}{
	"metrics": map[string]interface{}{
		"aggregation_dimensions": [][]string{{"InstanceId"}},
		"append_dimensions":      map[string]interface{}{"ImageId": "${aws:ImageId}", "InstanceId": "${aws:InstanceId}", "InstanceType": "${aws:InstanceType}", "AutoScalingGroupName": "${aws:AutoScalingGroupName}"},
		"metrics_collected": map[string]interface{}{
			"swap":    map[string]interface{}{"metrics_collection_interval": 60, "measurement": []string{"swap_used_percent"}},
			"cpu":     map[string]interface{}{"metrics_collection_interval": 60, "measurement": []string{"cpu_usage_idle", "cpu_usage_iowait", "cpu_usage_user", "cpu_usage_system"}, "resources": []string{"*"}, "totalcpu": false},
			"disk":    map[string]interface{}{"resources": []string{"*"}, "metrics_collection_interval": 60, "measurement": []string{"used_percent", "inodes_free"}},
			"diskio":  map[string]interface{}{"resources": []string{"*"}, "metrics_collection_interval": 60, "measurement": []string{"io_time", "write_bytes", "read_bytes", "writes", "reads"}},
			"mem":     map[string]interface{}{"metrics_collection_interval": 60, "measurement": []string{"mem_used_percent"}},
			"netstat": map[string]interface{}{"metrics_collection_interval": 60, "measurement": []string{"tcp_established", "tcp_time_wait"}}}}}

func TestProcessor_NextProcessor(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	conf := new(data.Config)

	// wantMonitorAnyHostMetrics?
	testutil.Type(inputChan, "2")
	nextProcessor := Processor.NextProcessor(ctx, conf)
	assert.Equal(t, linux.Processor, nextProcessor)
	assert.Equal(t, new(runtime.Context), ctx)
	assert.Equal(t, new(data.Config), conf)

	// wantMonitorAnyHostMetrics?
	// wantPerInstanceMetrics?
	// wantEC2TagDimensions?
	// wantEC2AggregateDimensions?
	// metricsCollectInterval?
	// whichDefaultConfig?
	testutil.Type(inputChan, "", "1", "", "", "", "4")
	nextProcessor = Processor.NextProcessor(ctx, conf)
	assert.Equal(t, question.Processor, nextProcessor)
	assert.Equal(t, true, ctx.WantPerInstanceMetrics)
	assert.Equal(t, true, ctx.WantEC2TagDimensions)
	assert.Equal(t, true, ctx.WantAggregateDimensions)
	assert.Equal(t, new(data.Config), conf)

	//basic metrics config
	ctx = new(runtime.Context)
	conf = new(data.Config)
	testutil.Type(inputChan, "", "1", "", "", "", "", "")
	nextProcessor = Processor.NextProcessor(ctx, conf)
	assert.Equal(t, linux.Processor, nextProcessor)

	_, confMap := conf.ToMap(ctx)
	assert.Equal(t, basicMetricsConf, confMap)

	//standard metrics config
	ctx = new(runtime.Context)
	conf = new(data.Config)
	testutil.Type(inputChan, "", "1", "", "", "", "2", "")
	nextProcessor = Processor.NextProcessor(ctx, conf)
	assert.Equal(t, linux.Processor, nextProcessor)

	_, confMap = conf.ToMap(ctx)
	assert.Equal(t, standardMetricsConf, confMap)

	//advanced metrics config
	ctx = new(runtime.Context)
	conf = new(data.Config)
	testutil.Type(inputChan, "", "1", "", "", "", "3", "")
	nextProcessor = Processor.NextProcessor(ctx, conf)
	assert.Equal(t, linux.Processor, nextProcessor)

	_, confMap = conf.ToMap(ctx)
	assert.Equal(t, advancedMetricsConf, confMap)

	//not satisfied with advanced config and restart with basic config
	ctx = new(runtime.Context)
	conf = new(data.Config)
	testutil.Type(inputChan, "", "1", "", "", "", "3", "2", "", "")
	nextProcessor = Processor.NextProcessor(ctx, conf)
	assert.Equal(t, linux.Processor, nextProcessor)

	_, confMap = conf.ToMap(ctx)
	assert.Equal(t, basicMetricsConf, confMap)
}
