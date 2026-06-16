// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mysql

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator(WithName("metrics"), WithIndex(0))
	assert.Equal(t, "mysql/metrics_0", tr.ID().String())

	tr = NewTranslator(WithName("events"), WithIndex(1))
	assert.Equal(t, "mysql/events_1", tr.ID().String())
}

func TestTranslator_Translate_Localhost(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:3306"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.mysql_credentials"),
		WithIsLocalhost(true),
		WithIndex(0),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	mysqlCfg := cfg.(*mysqlreceiver.Config)

	assert.Equal(t, "localhost:3306", mysqlCfg.Endpoint)
	assert.Equal(t, "cw_monitor", mysqlCfg.Username)
	assert.Equal(t, "/etc/.mysql_credentials", mysqlCfg.Passfile)
	assert.True(t, mysqlCfg.TLS.Insecure)
	assert.True(t, mysqlCfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled)
	assert.True(t, mysqlCfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled)
	assert.Equal(t, uint64(500), mysqlCfg.QuerySampleCollection.MaxRowsPerQuery)
	assert.Equal(t, uint64(200), mysqlCfg.TopQueryCollection.TopQueryCount)
	assert.Equal(t, uint64(5000), mysqlCfg.TopQueryCollection.MaxQuerySampleCount)
	assert.Equal(t, 1000, mysqlCfg.TopQueryCollection.QueryPlanCacheSize)
	assert.Equal(t, time.Hour, mysqlCfg.TopQueryCollection.QueryPlanCacheTTL)
	assert.Equal(t, 60*time.Second, mysqlCfg.TopQueryCollection.CollectionInterval)
}

func TestTranslator_Translate_CustomInterval(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:3306"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.mysql_credentials"),
		WithIsLocalhost(true),
		WithTopQueryInterval(30*time.Second),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.(*mysqlreceiver.Config).TopQueryCollection.CollectionInterval)
}

func TestTranslator_Translate_Remote(t *testing.T) {
	tr := NewTranslator(
		WithName("events"),
		WithEndpoint("db.example.com:3306"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.mysql_credentials"),
		WithCAFile("/etc/ssl/ca.pem"),
		WithIsLocalhost(false),
		WithIndex(0),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	mysqlCfg := cfg.(*mysqlreceiver.Config)

	assert.Equal(t, "db.example.com:3306", mysqlCfg.Endpoint)
	assert.False(t, mysqlCfg.TLS.Insecure)
	assert.Equal(t, "/etc/ssl/ca.pem", string(mysqlCfg.TLS.CAFile))
	assert.Equal(t, 60*time.Second, mysqlCfg.TopQueryCollection.CollectionInterval)
	assert.Equal(t, uint64(500), mysqlCfg.QuerySampleCollection.MaxRowsPerQuery)
	assert.True(t, mysqlCfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled)
	assert.True(t, mysqlCfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled)
}
