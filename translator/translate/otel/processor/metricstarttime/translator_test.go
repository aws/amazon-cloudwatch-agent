// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricstarttime

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator(common.WithName("test"))
	assert.EqualValues(t, "metric_start_time/test", tt.ID().String())

	got, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	gotCfg, ok := got.(*metricstarttimeprocessor.Config)
	require.True(t, ok)
	assert.Equal(t, "true_reset_point", gotCfg.Strategy)
	assert.Equal(t, metricstarttimeprocessor.NewFactory().CreateDefaultConfig(), gotCfg)
}

func TestTranslatorWithName(t *testing.T) {
	assert.EqualValues(t, "metric_start_time/otel_prometheus", NewTranslatorWithName("otel_prometheus").ID().String())
	assert.EqualValues(t, "metric_start_time", NewTranslatorWithName("").ID().String())
}
