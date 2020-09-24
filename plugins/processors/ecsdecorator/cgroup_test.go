// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsdecorator

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCGroupMountPoint(t *testing.T) {
	result, _ := getCGroupMountPoint("test/mountinfo")
	assert.Equal(t, "test", result, "Expected to be equal")
}

func TestGetCPUReservedFromShares(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")

	assert.Equal(t, int64(128), cgroup.getCPUReserved("test1", ""))
	assert.Equal(t, int64(128), cgroup.getCPUReserved("test4", "myCluster"))
}

func TestGetCPUReservedFromQuota(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	assert.Equal(t, int64(256), cgroup.getCPUReserved("test2", ""))
}

func TestGetCPUReservedFromBoth(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	assert.Equal(t, int64(256), cgroup.getCPUReserved("test3", ""))
}

func TestGetCPUReservedFromFalseTaskID(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	assert.Equal(t, int64(0), cgroup.getCPUReserved("fake", ""))
}

func TestGetMEMReservedFromTask(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	containers := []ECSContainer{}
	assert.Equal(t, int64(256), cgroup.getMEMReserved("test1", "", containers))
	assert.Equal(t, int64(256), cgroup.getMEMReserved("test3", "myCluster", containers))
}

func TestGetMEMReservedFromContainers(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	containers := []ECSContainer{ECSContainer{DockerId: "container1"}, ECSContainer{DockerId: "container2"}}
	assert.Equal(t, int64(384), cgroup.getMEMReserved("test2", "", containers))
}

func TestGetMEMReservedFromFalseTaskID(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	containers := []ECSContainer{ECSContainer{DockerId: "container1"}, ECSContainer{DockerId: "container2"}}
	assert.Equal(t, int64(0), cgroup.getMEMReserved("fake", "", containers))
}

func TestGetCGroupPathForTask(t *testing.T) {
	cgroupMount := "test"
	controller := "cpu"
	taskID := "test1"
	clusterName := "myCluster"
	result, _ := getCGroupPathForTask(cgroupMount, controller, taskID, clusterName)
	assert.Equal(t, path.Join(cgroupMount, controller, "ecs", taskID), result)

	taskID = "test4"
	result, _ = getCGroupPathForTask(cgroupMount, controller, taskID, clusterName)
	assert.Equal(t, path.Join(cgroupMount, controller, "ecs", clusterName, taskID), result)
}
