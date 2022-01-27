// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"github.com/docker/docker/pkg/testutil/assert"
	"testing"
	"time"
)

func TestUtils_parseDeploymentFromReplicaSet(t *testing.T) {
	assert.Equal(t, "", parseDeploymentFromReplicaSet("cloudwatch-agent"))
	assert.Equal(t, "cloudwatch-agent", parseDeploymentFromReplicaSet("cloudwatch-agent-42kcz"))
}

func TestUtils_parseCronJobFromJob(t *testing.T) {
	assert.Equal(t, "hello", parseCronJobFromJob("hello"+strconv.Itoa(time.Now().Unix()/60)))
	assert.Equal(t, "", parseCronJobFromJob("hello-"+strconv.Itoa(time.Now().Unix()/60)+"abc"))
	assert.Equal(t, "hello", parseCronJobFromJob("hello-"+strconv.Itoa(time.Now().Unix()/60)))
	assert.Equal(t, "", parseCronJobFromJob("hello-"+strconv.Itoa(time.Now().Unix()/60)+"-name"))
}
