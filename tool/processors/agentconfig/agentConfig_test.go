// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentconfig

import (
	"testing"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/testutil"

	"github.com/stretchr/testify/assert"
)

func TestProcessor_Process(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	conf := new(data.Config)

	testutil.Type(inputChan, "")
	Processor.Process(ctx, conf)
	assert.Equal(t, RUNASUSER_ROOT, conf.AgentConfig.Runasuser)

	testutil.Type(inputChan, "1")
	Processor.Process(ctx, conf)
	assert.Equal(t, RUNASUSER_ROOT, conf.AgentConfig.Runasuser)

	testutil.Type(inputChan, "2")
	Processor.Process(ctx, conf)
	assert.Equal(t, RUNASUSER_CWAGENT, conf.AgentConfig.Runasuser)

	testutil.Type(inputChan, "3", "testuser")
	Processor.Process(ctx, conf)
	assert.Equal(t, "testuser", conf.AgentConfig.Runasuser)
}
