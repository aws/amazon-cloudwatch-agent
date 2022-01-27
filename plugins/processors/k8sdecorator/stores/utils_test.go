// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"github.com/docker/docker/pkg/testutil/assert"
	"strconv"
	"testing"
	"time"
)

func TestUtils_parseDeploymentFromReplicaSet(t *testing.T) {
	assert.Equal(t, parseDeploymentFromReplicaSet("cloudwatch-agent"), "")
	assert.Equal(t, parseDeploymentFromReplicaSet("cloudwatch-agent-42kcz"), "cloudwatch-agent")
}

func TestUtils_parseCronJobFromJob(t *testing.T) {
	//For CronJobControllV2 which is after K8s v1.21
	assert.Equal(t, parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)), "hello")
	assert.Equal(t, parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)+"abc"), "")
	//For CronJob which is before K8s v1.21
	assert.Equal(t, parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)), "hello")
	assert.Equal(t, parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)+"-name"), "")
}
