// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func TestCollectorFeatureGateArgs(t *testing.T) {
	testCases := map[string]struct {
		register func(*testing.T, *featuregate.Registry)
		want     []string
	}{
		"GateRegistered": {
			register: func(t *testing.T, reg *featuregate.Registry) {
				_, err := reg.Register(profilesSupportGateID, featuregate.StageAlpha)
				require.NoError(t, err)
			},
			want: []string{"--feature-gates=+" + profilesSupportGateID},
		},
		// A gate that graduated to stable is still enableable, so keep passing it until
		// it is removed from the registry entirely.
		"GateStable": {
			register: func(t *testing.T, reg *featuregate.Registry) {
				_, err := reg.Register(profilesSupportGateID, featuregate.StageStable,
					featuregate.WithRegisterToVersion("v1.0.0"))
				require.NoError(t, err)
			},
			want: []string{"--feature-gates=+" + profilesSupportGateID},
		},
		// The gate graduated and was deleted upstream: emit nothing rather than an arg
		// the collector would reject at startup.
		"GateAbsent": {
			register: func(_ *testing.T, _ *featuregate.Registry) {},
			want:     nil,
		},
		// Registry.Set refuses to enable a deprecated gate, so it is as unusable as an
		// absent one.
		"GateDeprecated": {
			register: func(t *testing.T, reg *featuregate.Registry) {
				_, err := reg.Register(profilesSupportGateID, featuregate.StageDeprecated,
					featuregate.WithRegisterToVersion("v1.0.0"))
				require.NoError(t, err)
			},
			want: nil,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			reg := featuregate.NewRegistry()
			testCase.register(t, reg)

			got := collectorFeatureGateArgs(reg)
			assert.Equal(t, testCase.want, got)

			// The args are handed to the collector command verbatim, so check they parse
			// against the real feature gate flag rather than trusting the string above.
			flagSet := new(flag.FlagSet)
			reg.RegisterFlags(flagSet)
			require.NoError(t, flagSet.Parse(got))
		})
	}
}

func TestCollectorFeatureGateArgsEnablesGate(t *testing.T) {
	reg := featuregate.NewRegistry()
	gate, err := reg.Register(profilesSupportGateID, featuregate.StageAlpha)
	require.NoError(t, err)
	require.False(t, gate.IsEnabled())

	flagSet := new(flag.FlagSet)
	reg.RegisterFlags(flagSet)
	require.NoError(t, flagSet.Parse(collectorFeatureGateArgs(reg)))

	assert.True(t, gate.IsEnabled())
}

func TestCanEnableFeatureGate(t *testing.T) {
	reg := featuregate.NewRegistry()
	_, err := reg.Register("other.gate", featuregate.StageAlpha)
	require.NoError(t, err)

	assert.False(t, canEnableFeatureGate(reg, profilesSupportGateID))
	assert.True(t, canEnableFeatureGate(reg, "other.gate"))
}
