package ecsdecorator

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetCGroupMountPoint(t *testing.T) {
	result, _ := getCGroupMountPoint("test/mountinfo")
	assert.Equal(t, "test", result, "Expected to be equal")
}

func TestGetCPUReservedFromShares(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")

	assert.Equal(t, int64(128), cgroup.getCPUReserved("test1"))
}

func TestGetCPUReservedFromQuota(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	assert.Equal(t, int64(256), cgroup.getCPUReserved("test2"))
}

func TestGetCPUReservedFromBoth(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	assert.Equal(t, int64(256), cgroup.getCPUReserved("test3"))
}

func TestGetCPUReservedFromFalseTaskID(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	assert.Equal(t, int64(0), cgroup.getCPUReserved("fake"))
}

func TestGetMEMReservedFromTask(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	containers := []ECSContainer{}
	assert.Equal(t, int64(256), cgroup.getMEMReserved("test1", containers))
}

func TestGetMEMReservedFromContainers(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	containers := []ECSContainer{ECSContainer{DockerId: "container1"}, ECSContainer{DockerId: "container2"}}
	assert.Equal(t, int64(384), cgroup.getMEMReserved("test2", containers))
}

func TestGetMEMReservedFromFalseTaskID(t *testing.T) {
	cgroup := newCGroupScanner("test/mountinfo")
	containers := []ECSContainer{ECSContainer{DockerId: "container1"}, ECSContainer{DockerId: "container2"}}
	assert.Equal(t, int64(0), cgroup.getMEMReserved("fake", containers))
}
