// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"testing"

	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/file"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
)

func TestAddDefaultRelabelConfigs(t *testing.T) {
	scrapeConfigWithFileSD := &config.ScrapeConfig{
		JobName: "test-job-with-file-sd",
		ServiceDiscoveryConfigs: discovery.Configs{
			&file.SDConfig{},
		},
		RelabelConfigs:       []*relabel.Config{},
		MetricRelabelConfigs: []*relabel.Config{},
	}

	scrapeConfigs := []*config.ScrapeConfig{scrapeConfigWithFileSD}

	addDefaultRelabelConfigs(scrapeConfigs)

	assert.Len(t, scrapeConfigWithFileSD.RelabelConfigs, 10, "ScrapeConfig with file SD should have 10 default relabel configs")
	assert.Equal(t, "TaskClusterName", scrapeConfigWithFileSD.RelabelConfigs[0].TargetLabel)
	assert.Equal(t, "container_name", scrapeConfigWithFileSD.RelabelConfigs[1].TargetLabel)
	assert.Len(t, scrapeConfigWithFileSD.MetricRelabelConfigs, 1, "ScrapeConfig with file SD should have 1 default metric relabel config")
	assert.Equal(t, "TaskId", scrapeConfigWithFileSD.MetricRelabelConfigs[0].TargetLabel)
}

func TestAddDefaultRelabelConfigs_NoConfigsAdded(t *testing.T) {
	scrapeConfigWithoutFileSD := &config.ScrapeConfig{
		JobName:                 "test-job-without-file-sd",
		ServiceDiscoveryConfigs: discovery.Configs{},
		RelabelConfigs:          []*relabel.Config{},
		MetricRelabelConfigs:    []*relabel.Config{},
	}

	scrapeConfigs := []*config.ScrapeConfig{scrapeConfigWithoutFileSD}

	addDefaultRelabelConfigs(scrapeConfigs)

	assert.Len(t, scrapeConfigWithoutFileSD.RelabelConfigs, 0, "ScrapeConfig without file SD should have no relabel configs")
	assert.Len(t, scrapeConfigWithoutFileSD.MetricRelabelConfigs, 0, "ScrapeConfig without file SD should have no metric relabel configs")
}

func TestHasFileServiceDiscovery(t *testing.T) {
	// Test case 1: ScrapeConfig with file service discovery
	scrapeConfigWithFileSD := &config.ScrapeConfig{
		ServiceDiscoveryConfigs: discovery.Configs{
			&file.SDConfig{},
		},
	}

	// Test case 2: ScrapeConfig without file service discovery
	scrapeConfigWithoutFileSD := &config.ScrapeConfig{
		ServiceDiscoveryConfigs: discovery.Configs{},
	}

	assert.True(t, hasFileServiceDiscovery(scrapeConfigWithFileSD), "Should detect file service discovery")
	assert.False(t, hasFileServiceDiscovery(scrapeConfigWithoutFileSD), "Should not detect file service discovery when none present")
}
