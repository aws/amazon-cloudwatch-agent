// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestNewTranslatorsDeprecatedContainerInsights(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	withCI := confmap.NewFromStringMap(map[string]any{
		"opentelemetry": map[string]any{
			"collect": map[string]any{
				"container_insights": map[string]any{
					"role": "node",
					"logs": map[string]any{"enabled": true},
				},
			},
		},
	})
	got := NewTranslators(withCI)
	assert.Contains(t, buf.String(), "W! opentelemetry.collect.container_insights is deprecated and has no effect")

	// The deprecated section contributes no pipelines of its own.
	buf.Reset()
	want := NewTranslators(confmap.New())
	assert.ElementsMatch(t, want.Keys(), got.Keys())
	assert.Empty(t, buf.String())
}
