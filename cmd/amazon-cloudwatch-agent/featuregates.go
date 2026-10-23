// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"log"
	"strings"

	"go.opentelemetry.io/collector/featuregate"

	"github.com/aws/amazon-cloudwatch-agent/internal/featuregates"
)

// defaultEnabledFeatureGates are the collector feature gates the agent turns on at
// startup.
var defaultEnabledFeatureGates = []string{featuregates.ProfilesSupportGateID}

// collectorFeatureGateArgs builds the --feature-gates arguments for the collector
// command. A gate is only requested when the registry still holds it and still
// accepts being enabled: the collector fails startup on an unknown gate ID, so once
// a gate graduates and is deleted upstream, asking for it would turn a routine
// collector dependency bump into a fatal startup failure. Skipping it makes
// graduation a no-op instead.
func collectorFeatureGateArgs(reg *featuregate.Registry) []string {
	var gates []string
	for _, id := range defaultEnabledFeatureGates {
		if !featuregates.CanEnable(reg, id) {
			log.Printf("I! Feature gate %q cannot be enabled in this collector build, skipping", id)
			continue
		}
		gates = append(gates, "+"+id)
	}
	if len(gates) == 0 {
		return nil
	}
	return []string{"--feature-gates=" + strings.Join(gates, ",")}
}
