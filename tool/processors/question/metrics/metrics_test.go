// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration/linux"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/question/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestProcessor_Process(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	conf := new(data.Config)

	testutil.Type(inputChan, "", "", "", "", "")
	Processor.Process(ctx, conf)
	_, confMap := conf.ToMap(ctx)
	assert.Equal(t, map[string]interface{}{
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"disk":    map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"used_percent", "inodes_free"}},
				"diskio":  map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"io_time", "write_bytes", "read_bytes", "writes", "reads"}},
				"mem":     map[string]interface{}{"measurement": []string{"mem_used_percent"}},
				"net":     map[string]interface{}{"resources": []string{"*"}, "measurement": []string{"bytes_sent", "bytes_recv", "packets_sent", "packets_recv"}},
				"netstat": map[string]interface{}{"measurement": []string{"tcp_established", "tcp_time_wait"}},
				"swap":    map[string]interface{}{"measurement": []string{"swap_used_percent"}},
				"cpu":     map[string]interface{}{"measurement": []string{"cpu_usage_idle", "cpu_usage_iowait", "cpu_usage_steal", "cpu_usage_guest", "cpu_usage_user", "cpu_usage_system"}, "totalcpu": true}}}},
		confMap)
}

func TestProcessor_NextProcessor(t *testing.T) {
	ctx := new(runtime.Context)

	ctx.OsParameter = util.OsTypeWindows
	nextProcessor := Processor.NextProcessor(ctx, nil)
	assert.Equal(t, logs.Processor, nextProcessor)

	ctx.OsParameter = util.OsTypeLinux
	nextProcessor = Processor.NextProcessor(ctx, nil)
	assert.Equal(t, linux.Processor, nextProcessor)

	ctx.OsParameter = util.OsTypeDarwin
	nextProcessor = Processor.NextProcessor(ctx, nil)
	assert.Equal(t, linux.Processor, nextProcessor)
}
