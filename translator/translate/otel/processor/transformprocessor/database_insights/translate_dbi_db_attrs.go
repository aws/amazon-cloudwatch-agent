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

type resourceTranslator struct {
	instanceName string
	index        int
}

func NewResourceTranslator(instanceName string, index int) *resourceTranslator {
	return &resourceTranslator{instanceName: instanceName, index: index}
}

func (t *resourceTranslator) ID() component.ID {
	return component.MustNewIDWithName("transform", "dbi_resource_"+strconv.Itoa(t.index))
}

func (t *resourceTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	stmts := []interface{}{
		`set(resource.attributes["db.system.name"], "postgresql")`,
		fmt.Sprintf(`set(resource.attributes["db.instance.name"], "%s")`, t.instanceName),
	}
	context := map[string]interface{}{
		"context": "resource", "error_mode": "ignore", "statements": stmts,
	}
	cfg := &transformprocessor.Config{}
	if err := confmap.NewFromStringMap(map[string]interface{}{
		"error_mode":        "propagate",
		"metric_statements": []interface{}{context},
		"log_statements":    []interface{}{context},
	}).Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
