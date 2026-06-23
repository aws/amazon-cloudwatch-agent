// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

func TestSetMode_Azure(t *testing.T) {
	ResetContext()
	ctx := CurrentContext()

	ctx.SetMode(config.ModeAzureVM)
	assert.Equal(t, config.ModeAzureVM, ctx.Mode())
	assert.Equal(t, config.ShortModeAzureVM, ctx.ShortMode())
}

func TestSetMode_ExistingModesUnchanged(t *testing.T) {
	cases := map[string]struct {
		mode      string
		wantShort string
	}{
		"EC2":      {config.ModeEC2, config.ShortModeEC2},
		"OnPrem":   {config.ModeOnPrem, config.ShortModeOnPrem},
		"WithIRSA": {config.ModeWithIRSA, config.ShortModeWithIRSA},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ResetContext()
			ctx := CurrentContext()
			ctx.SetMode(tc.mode)
			assert.Equal(t, tc.mode, ctx.Mode())
			assert.Equal(t, tc.wantShort, ctx.ShortMode())
		})
	}
}

func TestSetKubernetesMode_AKS(t *testing.T) {
	ResetContext()
	ctx := CurrentContext()

	ctx.SetKubernetesMode(config.ModeAKS)
	assert.Equal(t, config.ModeAKS, ctx.KubernetesMode())
	assert.Equal(t, config.ShortModeAKS, ctx.ShortMode())
}

func TestSetKubernetesMode_ExistingModesUnchanged(t *testing.T) {
	cases := map[string]struct {
		mode      string
		wantShort string
	}{
		"EKS":       {config.ModeEKS, config.ShortModeEKS},
		"K8sEC2":    {config.ModeK8sEC2, config.ShortModeK8sEC2},
		"K8sOnPrem": {config.ModeK8sOnPrem, config.ShortModeK8sOnPrem},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ResetContext()
			ctx := CurrentContext()
			ctx.SetKubernetesMode(tc.mode)
			assert.Equal(t, tc.mode, ctx.KubernetesMode())
			assert.Equal(t, tc.wantShort, ctx.ShortMode())
		})
	}
}

func TestSetKubernetesMode_UnknownClears(t *testing.T) {
	ResetContext()
	ctx := CurrentContext()
	// A previously-set valid kubernetes mode is cleared by an unknown one.
	ctx.SetKubernetesMode(config.ModeAKS)
	ctx.SetKubernetesMode("not-a-real-mode")
	assert.Equal(t, "", ctx.KubernetesMode())
	// shortMode is intentionally NOT cleared by the default branch: the normal
	// non-Kubernetes path is SetMode(host) -> SetKubernetesMode("") and must keep
	// the host shortMode set by SetMode. Here no host mode was set, so it retains
	// the last valid kubernetes shortMode (ShortModeAKS).
	assert.Equal(t, config.ShortModeAKS, ctx.ShortMode())
}

func TestSetKubernetesMode_EmptyPreservesHostShortMode(t *testing.T) {
	ResetContext()
	ctx := CurrentContext()
	// Mimic the real startup ordering: host mode first, then the empty
	// (non-Kubernetes) kubernetes mode. The host shortMode must survive.
	ctx.SetMode(config.ModeAzureVM)
	ctx.SetKubernetesMode("")
	assert.Equal(t, "", ctx.KubernetesMode())
	assert.Equal(t, config.ShortModeAzureVM, ctx.ShortMode())
}
