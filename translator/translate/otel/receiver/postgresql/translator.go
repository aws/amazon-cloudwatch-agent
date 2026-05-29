// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package postgresql

import (
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type Option func(*translator)

type translator struct {
	factory             receiver.Factory
	name                string
	endpoint            string
	username            string
	passfile            string
	caFile              string
	isLocalhost         bool
	index               int
	querySampleInterval time.Duration
	maxRowsPerQuery     int64
}

func WithName(name string) Option   { return func(t *translator) { t.name = name } }
func WithEndpoint(ep string) Option { return func(t *translator) { t.endpoint = ep } }
func WithUsername(u string) Option  { return func(t *translator) { t.username = u } }
func WithPassfile(p string) Option  { return func(t *translator) { t.passfile = p } }
func WithCAFile(ca string) Option   { return func(t *translator) { t.caFile = ca } }
func WithIsLocalhost(b bool) Option { return func(t *translator) { t.isLocalhost = b } }
func WithIndex(i int) Option        { return func(t *translator) { t.index = i } }
func WithQuerySampleInterval(d time.Duration) Option {
	return func(t *translator) { t.querySampleInterval = d }
}
func WithMaxRowsPerQuery(n int64) Option { return func(t *translator) { t.maxRowsPerQuery = n } }

func NewTranslator(opts ...Option) common.ComponentTranslator {
	t := &translator{
		factory:             postgresqlreceiver.NewFactory(),
		name:                "metrics",
		querySampleInterval: time.Second,
		maxRowsPerQuery:     500,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.MustNewIDWithName("postgresql", t.name+"_"+strconv.Itoa(t.index))
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*postgresqlreceiver.Config)

	cfg.Endpoint = t.endpoint
	cfg.Username = t.username
	cfg.Passfile = t.passfile
	cfg.Transport = "tcp"

	if t.isLocalhost {
		cfg.Insecure = true
		cfg.InsecureSkipVerify = true
	} else {
		cfg.CAFile = t.caFile
		cfg.InsecureSkipVerify = false
	}

	cfg.Metrics.PostgresqlFunctionCalls.Enabled = false
	cfg.Metrics.PostgresqlTupDeleted.Enabled = false
	cfg.Metrics.PostgresqlTupFetched.Enabled = false
	cfg.Metrics.PostgresqlTupInserted.Enabled = false
	cfg.Metrics.PostgresqlTupReturned.Enabled = false
	cfg.Metrics.PostgresqlTupUpdated.Enabled = false
	cfg.Metrics.PostgresqlWalDelay.Enabled = false

	cfg.Events.DbServerQuerySample.Enabled = true
	cfg.Events.DbServerTopQuery.Enabled = true

	cfg.Enabled = true
	cfg.QuerySampleCollection.CollectionInterval = t.querySampleInterval
	cfg.QuerySampleCollection.MaxRowsPerQuery = t.maxRowsPerQuery

	cfg.TopQueryCollection.CollectionInterval = 60 * time.Second
	cfg.TopNQuery = 5000
	cfg.TopQueryCollection.MaxRowsPerQuery = 200
	cfg.MaxExplainEachInterval = 0

	return cfg, nil
}
