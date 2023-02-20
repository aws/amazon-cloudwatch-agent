// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Prometheus <labelname> definition: a string matching the regular expression [a-zA-Z_][a-zA-Z0-9_]*
// Regex pattern to filter out invalid labels
const (
	prometheusLabelNamePattern = "^[a-zA-Z_][a-zA-Z0-9_]*$"
)

type PrometheusTarget struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

type TargetsExportProcessor struct {
	config *ServiceDiscoveryConfig
	stats  *ProcessorStats

	dockerLabelRegex  *regexp.Regexp
	tmpResultFilePath string
}

func NewTargetsExportProcessor(sdConfig *ServiceDiscoveryConfig, s *ProcessorStats) *TargetsExportProcessor {
	return &TargetsExportProcessor{
		config:            sdConfig,
		stats:             s,
		dockerLabelRegex:  regexp.MustCompile(prometheusLabelNamePattern),
		tmpResultFilePath: sdConfig.ResultFile + "_temp",
	}
}

func (p *TargetsExportProcessor) Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	// Dedup Key for Targets: target + metricsPath
	// e.g. 10.0.0.28:9404/metrics
	//      10.0.0.28:9404/stats/metrics
	targets := make(map[string]*PrometheusTarget)
	for _, t := range taskList {
		t.ExporterInformation(p.config, p.dockerLabelRegex, targets)
	}

	targetsArr := make([]*PrometheusTarget, 0, len(targets))
	for _, value := range targets {
		targetsArr = append(targetsArr, value)
	}

	m, err := yaml.Marshal(targetsArr)
	if err != nil {
		return nil, newServiceDiscoveryError("Fail to marshal Prometheus Targets!", &err)
	}
	p.stats.AddStatsCount(ExporterDiscoveredTargetCount, len(targetsArr))

	err = os.WriteFile(p.tmpResultFilePath, m, 0644)
	if err != nil {
		return nil, newServiceDiscoveryError(fmt.Sprintf("Fail to write Prometheus targets into file: %v", p.tmpResultFilePath), &err)
	}
	err = os.Rename(p.tmpResultFilePath, p.config.ResultFile)
	if err != nil {
		os.Remove(p.tmpResultFilePath)
		return nil, newServiceDiscoveryError(fmt.Sprintf("Fail to rename tmp result file %v to: %v", p.tmpResultFilePath, p.config.ResultFile), &err)
	}

	return nil, nil
}

func (p *TargetsExportProcessor) ProcessorName() string {
	return "TargetsExportProcessor"
}
