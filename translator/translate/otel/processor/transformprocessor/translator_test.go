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

func TestDbiFixStartTimeTranslate(t *testing.T) {
	transl := NewTranslatorWithName(common.DbiTransformFixStartTime)
	assert.Equal(t, "transform/dbi_fix_start_time", transl.ID().String())

	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)
	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Equal(t, "datapoint", string(actualCfg.MetricStatements[0].Context))
	require.Len(t, actualCfg.MetricStatements[0].Statements, 7)
	assert.Equal(t, "set(datapoint.start_time_unix_nano, datapoint.time_unix_nano) where datapoint.start_time_unix_nano == 0", actualCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `replace_match(datapoint.attributes["user.name"], "", "unknown")`, actualCfg.MetricStatements[0].Statements[6])
}

func TestDbiResourceTranslate(t *testing.T) {
	stmts := []string{
		`set(resource.attributes["db.system.name"], "postgresql")`,
		`set(resource.attributes["db.instance.name"], "my-db")`,
	}
	transl := NewTranslatorWithName(common.DbiTransformResource+"_0",
		WithMetricStatements(stmts),
		WithLogStatements(stmts),
	)
	assert.Equal(t, "transform/dbi_resource_0", transl.ID().String())

	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.MetricStatements, 1)
	assert.Equal(t, "resource", string(actualCfg.MetricStatements[0].Context))
	require.Len(t, actualCfg.MetricStatements[0].Statements, 2)
	assert.Equal(t, `set(resource.attributes["db.system.name"], "postgresql")`, actualCfg.MetricStatements[0].Statements[0])
	assert.Equal(t, `set(resource.attributes["db.instance.name"], "my-db")`, actualCfg.MetricStatements[0].Statements[1])

	require.Len(t, actualCfg.LogStatements, 1)
	assert.Equal(t, actualCfg.MetricStatements[0].Statements, actualCfg.LogStatements[0].Statements)
}

func TestDbiLogDestinationTranslate(t *testing.T) {
	stmts := []string{
		`set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/postgresql/raw-events")`,
		`set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "my-db"], "/"))`,
	}
	transl := NewTranslatorWithName(common.DbiTransformLogs+"_raw-events_0", WithLogStatements(stmts))
	assert.Equal(t, "transform/dbi_logs_raw-events_0", transl.ID().String())

	cfg, err := transl.Translate(nil)
	require.NoError(t, err)
	actualCfg := cfg.(*transformprocessor.Config)

	require.Len(t, actualCfg.LogStatements, 1)
	assert.Equal(t, "resource", string(actualCfg.LogStatements[0].Context))
	assert.Equal(t, "propagate", string(actualCfg.LogStatements[0].ErrorMode))
	require.Len(t, actualCfg.LogStatements[0].Statements, 2)
	assert.Equal(t, `set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/postgresql/raw-events")`, actualCfg.LogStatements[0].Statements[0])
	assert.Equal(t, `set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "my-db"], "/"))`, actualCfg.LogStatements[0].Statements[1])
	assert.Empty(t, actualCfg.MetricStatements)
}
