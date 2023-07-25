// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package serialization

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/ssm"
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
	nextProcessor := Processor.NextProcessor(nil, nil)
	assert.Equal(t, ssm.Processor, nextProcessor)
}
