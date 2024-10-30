// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package receiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestTranslator(t *testing.T) {
	factory := receivertest.NewNopFactory()
	got := NewDefaultTranslator(factory)
	require.Equal(t, "nop", got.ID().String())
	cfg, err := got.Translate(nil)
	require.NoError(t, err)
	require.Equal(t, factory.CreateDefaultConfig(), cfg)
}
