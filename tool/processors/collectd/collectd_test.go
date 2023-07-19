// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/defaultConfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
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
	collectdConf := conf.MetricsConf().Collection().CollectD
	assert.NotNil(t, collectdConf)
	assert.Equal(t, 60, collectdConf.MetricsAggregationInterval)
}

func TestProcessor_NextProcessor(t *testing.T) {
	ctx := new(runtime.Context)
	ctx.OsParameter = util.OsTypeWindows
	assert.Equal(t, migration.Processor, Processor.NextProcessor(ctx, nil))

	ctx.OsParameter = util.OsTypeLinux
	assert.Equal(t, defaultConfig.Processor, Processor.NextProcessor(ctx, nil))

	ctx.OsParameter = util.OsTypeDarwin
	assert.Equal(t, defaultConfig.Processor, Processor.NextProcessor(ctx, nil))
}
