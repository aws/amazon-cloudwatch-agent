// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin
// +build linux darwin

package cadvisor

import (
	"flag"
	"log"
	"net/http"
	"time"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/google/cadvisor/cache/memory"
	cadvisormetrics "github.com/google/cadvisor/container"
	"github.com/google/cadvisor/container/containerd"
	"github.com/google/cadvisor/container/crio"
	"github.com/google/cadvisor/container/docker"
	"github.com/google/cadvisor/container/mesos"
	"github.com/google/cadvisor/container/systemd"
	cinfo "github.com/google/cadvisor/info/v1"
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/utils/sysfs"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

// The amount of time for which to keep stats in memory.
const statsCacheDuration = 2 * time.Minute

// Max collection interval, it is not meaningful if allowDynamicHousekeeping = false
const maxHousekeepingInterval = 15 * time.Second

// When allowDynamicHousekeeping is true, the collection interval is floating between 1s(default) to maxHousekeepingInterval
const allowDynamicHousekeeping = true

const defaultHousekeepingInterval = 10 * time.Second

type Cadvisor struct {
	manager               manager.Manager
	Mode                  string `toml:"mode"`
	ContainerOrchestrator string `toml:"container_orchestrator"`
}

func overrideCadvisorFlagDefault() {
	flagOverrides := map[string]string{
		// Override the default cAdvisor housekeeping interval.
		"housekeeping_interval": defaultHousekeepingInterval.String(),
		// override other defaults (in future)
	}
	for name, defaultValue := range flagOverrides {
		if f := flag.Lookup(name); f != nil {
			f.DefValue = defaultValue
			f.Value.Set(defaultValue)
		} else {
			log.Printf("E! Expected cAdvisor flag %q not found", name)
		}
	}
}

func init() {
	c := &Cadvisor{}
	overrideCadvisorFlagDefault()
	inputs.Add("cadvisor", func() telegraf.Input {
		return c
	})
}

func (c *Cadvisor) SampleConfig() string {
	return ""
}

func (c *Cadvisor) Description() string {
	return "Collect metrics through Cadvisor"
}

func (c *Cadvisor) isDetailMode() bool {
	return c.Mode == "detail"
}

func (c *Cadvisor) Gather(acc telegraf.Accumulator) error {
	log.Printf("D! collect data from cadvisor...")
	var infos []*cinfo.ContainerInfo
	var err error

	if c.manager == nil && c.initManager() != nil {
		log.Panic("E! Cannot initiate manager")
	}

	req := &cinfo.ContainerInfoRequest{
		NumStats: 1,
	}
	infos, err = c.manager.SubcontainersInfo("/", req)
	if err != nil {
		log.Printf("E! GetContainerInfo failed %v", err)
		return err
	} else {
		log.Printf("D! size of containers stats %d", len(infos))
		results := processContainers(infos, c.isDetailMode(), c.ContainerOrchestrator)
		for _, cadvisorMetric := range results {
			acc.AddFields("cadvisor", cadvisorMetric.GetFields(), cadvisorMetric.GetAllTags())
		}
	}
	return nil
}

func (c *Cadvisor) initManager() error {
	sysFs := sysfs.NewRealSysFs()
	includedMetrics := cadvisormetrics.MetricSet{
		cadvisormetrics.CpuUsageMetrics:     struct{}{},
		cadvisormetrics.MemoryUsageMetrics:  struct{}{},
		cadvisormetrics.DiskIOMetrics:       struct{}{},
		cadvisormetrics.NetworkUsageMetrics: struct{}{},
		cadvisormetrics.DiskUsageMetrics:    struct{}{},
	}
	var cgroupRoots []string
	if c.ContainerOrchestrator == EKS {
		cgroupRoots = []string{"/kubepods"}
	}

	// Create and start the cAdvisor container manager.
	m, err := manager.New(memory.New(statsCacheDuration, nil), sysFs, maxHousekeepingInterval, allowDynamicHousekeeping, includedMetrics, http.DefaultClient, cgroupRoots)
	if err != nil {
		log.Println("E! manager allocate failed, ", err)
		return err
	}
	cadvisormetrics.RegisterPlugin("containerd", containerd.NewPlugin())
	cadvisormetrics.RegisterPlugin("crio", crio.NewPlugin())
	cadvisormetrics.RegisterPlugin("docker", docker.NewPlugin())
	cadvisormetrics.RegisterPlugin("mesos", mesos.NewPlugin())
	cadvisormetrics.RegisterPlugin("systemd", systemd.NewPlugin())
	c.manager = m
	err = c.manager.Start()
	if err != nil {
		log.Println("E! manager start failed, ", err)
		return err
	}
	return nil
}
