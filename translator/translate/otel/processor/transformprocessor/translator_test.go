// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	_ "embed"
	"path/filepath"
	"sort"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestContainerInsightsJmx(t *testing.T) {
	transl := NewTranslatorWithName(common.PipelineNameContainerInsightsJmx).(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, "transform_jmx_config.yaml")
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)
	assert.Equal(t, len(expectedCfg.MetricStatements), len(actualCfg.MetricStatements))
}

func TestEfaTranslate(t *testing.T) {
	transl := NewTranslatorWithName(common.PipelineNameHostDeltaMetrics).(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, "transform_efa_config.yaml")
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)
	assert.Equal(t, len(expectedCfg.MetricStatements), len(actualCfg.MetricStatements))
	assert.Equal(t, string(actualCfg.ErrorMode), "propagate")
	// Verify EFA attribute renaming statements are present
	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Contains(t, actualCfg.MetricStatements[0].Statements, `set(attributes["device"], attributes["aws.efa.device"]) where attributes["aws.efa.device"] != nil`)
	assert.Contains(t, actualCfg.MetricStatements[0].Statements, `set(attributes["port"], attributes["aws.efa.port"]) where attributes["aws.efa.port"] != nil`)
	assert.Contains(t, actualCfg.MetricStatements[0].Statements, `set(attributes["eniId"], attributes["aws.efa.eni.id"]) where attributes["aws.efa.eni.id"] != nil`)
}

func TestJmxTranslate(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_LINUX)
	transl := NewTranslatorWithName(common.PipelineNameJmx + "/drop").(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, "transform_jmx_drop_config.yaml")
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)

	// sort the statements for consistency
	assert.Len(t, expectedCfg.MetricStatements, 1)
	assert.Len(t, actualCfg.MetricStatements, 1)
	sort.Strings(expectedCfg.MetricStatements[0].Statements)
	sort.Strings(actualCfg.MetricStatements[0].Statements)
	assert.Equal(t, string(actualCfg.ErrorMode), "propagate")
}
