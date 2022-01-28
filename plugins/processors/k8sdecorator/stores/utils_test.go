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
	assert.Equal(t, "", parseDeploymentFromReplicaSet("cloudwatch-agent"))
	assert.Equal(t, "cloudwatch-agent", parseDeploymentFromReplicaSet("cloudwatch-agent-42kcz"))
}

func TestUtils_parseCronJobFromJob(t *testing.T) {
	//For CronJobControllV2 which is after K8s v1.21
	assert.Equal(t, "hello", parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)))
	assert.Equal(t, "", parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)+"abc"))
	assert.Equal(t, "", parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix()/60, 10)+"-name"))

	//For CronJob which is before K8s v1.21
	assert.Equal(t, "hello", parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix(), 10)))
	assert.Equal(t, "", parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix(), 10)+"-name"))
	assert.Equal(t, "", parseCronJobFromJob("hello-"+strconv.FormatInt(time.Now().Unix(), 10)+"&64"))
	assert.Equal(t, "", parseCronJobFromJob("hello"+strconv.FormatInt(time.Now().Unix(), 10)+"-/@89@"))
}
