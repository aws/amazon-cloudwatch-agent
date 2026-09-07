// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package featuregates

import (
	"go.opentelemetry.io/collector/featuregate"
)

const ProfilesSupportGateID = "service.profilesSupport"

var DefaultEnabled = []string{ProfilesSupportGateID}

func CanEnable(reg *featuregate.Registry, id string) bool {
	var enableable bool
	reg.VisitAll(func(g *featuregate.Gate) {
		if g.ID() == id {
			enableable = g.Stage() != featuregate.StageDeprecated
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
