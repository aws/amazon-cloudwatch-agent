// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package postgresql

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator(WithName("metrics"), WithIndex(0))
	assert.Equal(t, "postgresql/metrics_0", tr.ID().String())

	tr = NewTranslator(WithName("events"), WithIndex(1))
	assert.Equal(t, "postgresql/events_1", tr.ID().String())
}

func TestTranslator_Translate_Defaults(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:5432"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.pgpass"),
		WithIsLocalhost(true),
		WithIndex(0),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	pgCfg := cfg.(*postgresqlreceiver.Config)

	assert.Equal(t, "localhost:5432", pgCfg.Endpoint)
	assert.Equal(t, "cw_monitor", pgCfg.Username)
	assert.Equal(t, "/etc/.pgpass", pgCfg.Passfile)
	assert.True(t, pgCfg.ClientConfig.Insecure)
	assert.True(t, pgCfg.ClientConfig.InsecureSkipVerify)
	assert.Equal(t, time.Second, pgCfg.QuerySampleCollection.CollectionInterval)
	assert.Equal(t, int64(500), pgCfg.QuerySampleCollection.MaxRowsPerQuery)
	assert.Equal(t, int64(5000), pgCfg.TopQueryCollection.TopNQuery)
	assert.Equal(t, int64(200), pgCfg.TopQueryCollection.MaxRowsPerQuery)
}

func TestTranslator_Translate_Events(t *testing.T) {
	tr := NewTranslator(
		WithName("events"),
		WithEndpoint("db.example.com:5432"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.pgpass"),
		WithCAFile("/etc/ssl/ca.pem"),
		WithIsLocalhost(false),
		WithIndex(0),
		WithQuerySampleInterval(60*time.Second),
		WithMaxRowsPerQuery(500),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	pgCfg := cfg.(*postgresqlreceiver.Config)

	assert.Equal(t, "db.example.com:5432", pgCfg.Endpoint)
	assert.False(t, pgCfg.ClientConfig.Insecure)
	assert.False(t, pgCfg.ClientConfig.InsecureSkipVerify)
	assert.Equal(t, "/etc/ssl/ca.pem", string(pgCfg.ClientConfig.CAFile))
	assert.Equal(t, 60*time.Second, pgCfg.QuerySampleCollection.CollectionInterval)
	assert.Equal(t, int64(500), pgCfg.QuerySampleCollection.MaxRowsPerQuery)
}

func TestTranslator_Translate_CustomInterval(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:5432"),
		WithUsername("u"),
		WithPassfile("p"),
		WithIsLocalhost(true),
		WithQuerySampleInterval(5*time.Second),
		WithMaxRowsPerQuery(200),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	pgCfg := cfg.(*postgresqlreceiver.Config)

	assert.Equal(t, 5*time.Second, pgCfg.QuerySampleCollection.CollectionInterval)
	assert.Equal(t, int64(200), pgCfg.QuerySampleCollection.MaxRowsPerQuery)
}
