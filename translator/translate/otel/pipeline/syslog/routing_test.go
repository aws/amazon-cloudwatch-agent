// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
)

// TestRoutingConnectorTranslate_RoundTripsThroughUnmarshal builds a routing
// connector config the way the syslog pipeline does, then round-trips it
// through confmap Marshal -> Unmarshal into a fresh default config. This
// reproduces the collector's startup decode path, where each table entry's
// "action" is decoded via Action.UnmarshalText.
//
// Regression guard: on OTel v0.150 the routingconnector requires a valid
// "action" ("move" or "copy") per table entry. An empty action serializes as
// action: "" and fails UnmarshalText with `invalid Action string:` at agent
// startup. This test exercises Unmarshal (not just Validate, which would
// silently default an empty action to move) so the failure is caught here.
func TestRoutingConnectorTranslate_RoundTripsThroughUnmarshal(t *testing.T) {
	defaultID := pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_default")
	rule0 := pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_rule_0")
	rule1 := pipeline.NewIDWithName(pipeline.SignalLogs, "syslog_0_rule_1")

	tr := newRoutingConnectorTranslator("syslog_0",
		[]pipeline.ID{defaultID},
		[]routingTableEntry{
			{condition: `IsMatch(attributes["hostname"], "^web-.*$")`, pipelines: []pipeline.ID{rule0}},
			{condition: `attributes["facility"] == 4`, pipelines: []pipeline.ID{rule1}},
		},
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Serialize the produced config, then decode it back into a fresh default
	// config — the same Marshal/Unmarshal path the collector uses at startup.
	conf := confmap.New()
	require.NoError(t, conf.Marshal(cfg))

	fresh := routingconnector.NewFactory().CreateDefaultConfig()
	require.NoError(t, conf.Unmarshal(fresh),
		"routing connector config must decode via UnmarshalText; an empty action fails here")

	if v, ok := fresh.(interface{ Validate() error }); ok {
		assert.NoError(t, v.Validate())
	}
}
