// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightscommon

import (
	"log"
	"os"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type NodeCapacity struct {
	MemCapacity int64
	CPUCapacity int64
}

func NewNodeCapacity() *NodeCapacity {
	if _, err := os.Lstat("/rootfs/proc"); os.IsNotExist(err) {
		log.Panic("E! /rootfs/proc does not exist")
	}
	if err := os.Setenv(GoPSUtilProcDirEnv, "/rootfs/proc"); err != nil {
		log.Printf("E! NodeCapacity cannot set goPSUtilProcDirEnv to /rootfs/proc %v", err)
	}
	nc := &NodeCapacity{}
	nc.parseCpu()
	nc.parseMemory()
	return nc
}

func (n *NodeCapacity) parseMemory() {
	if memStats, err := mem.VirtualMemory(); err == nil {
		n.MemCapacity = int64(memStats.Total)
	} else {
		// If any error happen, then there will be no mem utilization metrics
		log.Printf("E! NodeCapacity cannot get memStats from psUtil %v", err)
	}
}

func (n *NodeCapacity) parseCpu() {
	if cpuInfos, err := cpu.Info(); err == nil {
		numCores := len(cpuInfos)
		n.CPUCapacity = int64(numCores)
	} else {
		// If any error happen, then there will be no cpu utilization metrics
		log.Printf("E! NodeCapacity cannot get cpuInfo from psUtil %v", err)
	}
}
