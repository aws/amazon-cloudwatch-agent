package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/featuregate"
)

func TestProfilesSupportGateInGlobalRegistry(t *testing.T) {
	assert.True(t, canEnableFeatureGate(featuregate.GlobalRegistry(), profilesSupportGateID),
		"service.profilesSupport must be enableable in the collector build the agent embeds")
	args := collectorFeatureGateArgs(featuregate.GlobalRegistry())
	assert.Equal(t, []string{"--feature-gates=+" + profilesSupportGateID}, args)
}
