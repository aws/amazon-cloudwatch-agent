// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package database_insights

import (
	"fmt"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

type logsRoutingTranslator struct {
	instanceName string
	streamType   string
	index        int
}

func NewLogsRoutingTranslator(instanceName, streamType string, index int) *logsRoutingTranslator {
	return &logsRoutingTranslator{instanceName: instanceName, streamType: streamType, index: index}
}

func (t *logsRoutingTranslator) ID() component.ID {
	return component.MustNewIDWithName("transform", "dbi_logs_"+t.streamType+"_"+strconv.Itoa(t.index))
}

func (t *logsRoutingTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	stmts := []interface{}{
		fmt.Sprintf(`set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/postgresql/%s")`, t.streamType),
		fmt.Sprintf(`set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "%s"], "/"))`, t.instanceName),
	}
	cfg := &transformprocessor.Config{}
	if err := confmap.NewFromStringMap(map[string]interface{}{
		"error_mode": "propagate",
		"log_statements": []interface{}{
			map[string]interface{}{
				"context": "resource", "error_mode": "ignore", "statements": stmts,
			},
		},
	}).Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
