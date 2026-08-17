// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package spanmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator("opentelemetry")
	assert.Equal(t, "spanmetrics/opentelemetry", tr.ID().String())
}

func TestTranslator_Translate(t *testing.T) {
	tr := NewTranslator("opentelemetry")
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}
