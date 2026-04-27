// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package postgresqlreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func setFeatureGateForTest(tb testing.TB, gate *featuregate.Gate, enabled bool) func() {
	originalValue := gate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	return func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(gate.ID(), originalValue))
	}
}
