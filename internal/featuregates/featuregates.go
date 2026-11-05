// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package featuregates

import (
	"go.opentelemetry.io/collector/featuregate"
)

// ProfilesSupportGateID is the collector gate that unlocks profiles pipelines. The
// agent enables it so operators get profiles support without having to pass
// --feature-gates themselves.
const ProfilesSupportGateID = "service.profilesSupport"

// CanEnable reports whether reg holds a gate with the given id that can be
// enabled. A deprecated gate is treated the same as a missing one because
// featuregate.Registry.Set rejects enabling it. A stable gate is already on and
// Set prints a notice when asked to enable it again.
func CanEnable(reg *featuregate.Registry, id string) bool {
	var enableable bool
	reg.VisitAll(func(g *featuregate.Gate) {
		if g.ID() == id {
			enableable = g.Stage() != featuregate.StageDeprecated && g.Stage() != featuregate.StageStable
		}
	})
	return enableable
}

func Enable(reg *featuregate.Registry, id string) error {
	if !CanEnable(reg, id) {
		return nil
	}
	return reg.Set(id, true)
}
