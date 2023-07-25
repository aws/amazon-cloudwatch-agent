// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package question

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/question/metrics"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestProcessor_Process(t *testing.T) {
	ctx := new(runtime.Context)
	conf := new(data.Config)

	Processor.Process(ctx, conf)
	assert.Equal(t, new(runtime.Context), ctx)
	assert.Equal(t, new(data.Config), conf)
}

func TestProcessor_NextProcessor(t *testing.T) {
	ctx := new(runtime.Context)
	conf := new(data.Config)

	nextProcessor := Processor.NextProcessor(ctx, conf)
	assert.Equal(t, metrics.Processor, nextProcessor)
	assert.Equal(t, new(data.Config), conf)
}
