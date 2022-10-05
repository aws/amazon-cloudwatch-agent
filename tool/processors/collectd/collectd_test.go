// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/defaultConfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/migration"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/testutil"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
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
