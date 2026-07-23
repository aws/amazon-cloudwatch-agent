// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mysql

import (
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type Option func(*translator)

type translator struct {
	factory          receiver.Factory
	name             string
	endpoint         string
	username         string
	passfile         string
	index            int
	topQueryInterval time.Duration
}

func WithName(name string) Option   { return func(t *translator) { t.name = name } }
func WithEndpoint(ep string) Option { return func(t *translator) { t.endpoint = ep } }
func WithUsername(u string) Option  { return func(t *translator) { t.username = u } }
func WithPassfile(p string) Option  { return func(t *translator) { t.passfile = p } }
func WithIndex(i int) Option        { return func(t *translator) { t.index = i } }
func WithTopQueryInterval(d time.Duration) Option {
	return func(t *translator) { t.topQueryInterval = d }
}

func NewTranslator(opts ...Option) common.ComponentTranslator {
	t := &translator{
		factory:          mysqlreceiver.NewFactory(),
		name:             "metrics",
		topQueryInterval: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.MustNewIDWithName("mysql", t.name+"_"+strconv.Itoa(t.index))
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*mysqlreceiver.Config)

	cfg.Endpoint = t.endpoint
	cfg.Username = t.username
	cfg.Passfile = t.passfile
	cfg.Transport = confignet.TransportTypeTCP

	// DBI only supports local MySQL instances, so the connection does not use
	// TLS. Remote (TLS) connections are intentionally not supported.
	cfg.TLS = configtls.ClientConfig{
		Insecure: true,
	}

	// Enable metrics that are disabled by default in the receiver but required
	// by Database Insights.
	cfg.MetricsBuilderConfig.Metrics.MysqlTableSize.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.MysqlReplicaSQLDelay.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.MysqlReplicaTimeBehindSource.Enabled = true

	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true

	cfg.QuerySampleCollection.MaxRowsPerQuery = 500

	cfg.TopQueryCollection.CollectionInterval = t.topQueryInterval
	cfg.TopQueryCollection.TopQueryCount = 200
	cfg.TopQueryCollection.MaxQuerySampleCount = 5000
	cfg.TopQueryCollection.QueryPlanCacheSize = 1000
	cfg.TopQueryCollection.QueryPlanCacheTTL = time.Hour

	return cfg, nil
}
