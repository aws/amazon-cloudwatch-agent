// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// source(Apache 2.0): https://github.com/DataDog/datadog-agent/blob/main/pkg/collector/python/datadog_agent.go

// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package postgresqlreceiver // import "github.com/aws/amazon-cloudwatch-agent/receiver/postgresqlreceiver"

import (
	"sync"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

var (
	obfuscator       *obfuscate.Obfuscator
	obfuscatorLoader sync.Once
)

// defaultSQLPlanObfuscateSettings for obfuscating SQL execution plans
var defaultSQLPlanObfuscateSettings = obfuscate.JSONConfig{
	Enabled:            true,
	ObfuscateSQLValues: []string{"Cache Key", "Conflict Filter", "Function Call", "Filter", "Hash Cond", "Index Cond", "Join Filter", "Merge Cond", "Output", "Recheck Cond", "Repeatable Seed", "Sampling Parameters", "TID Cond"},
	KeepValues:         []string{"Plan Rows", "Plan Width", "Startup Cost", "Total Cost", "Actual Loops", "Actual Rows", "Actual Startup Time", "Actual Total Time", "Alias", "Async Capable", "Node Type", "Parallel Aware", "Parent Relationship", "Relation Name", "Scan Direction", "Index Name", "Join Type", "Sort Key", "Sort Method", "Strategy", "Workers Planned", "Workers Launched"},
}

// defaultSQLPlanNormalizeSettings for normalizing SQL execution plans
var defaultSQLPlanNormalizeSettings = obfuscate.JSONConfig{
	Enabled:            true,
	ObfuscateSQLValues: defaultSQLPlanObfuscateSettings.ObfuscateSQLValues,
	KeepValues:         defaultSQLPlanObfuscateSettings.KeepValues,
}

// lazyInitObfuscator initializes the obfuscator the first time it is used.
func lazyInitObfuscator() *obfuscate.Obfuscator {
	obfuscatorLoader.Do(func() {
		obfuscator = obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				DBMS:         "postgresql",
				KeepSQLAlias: true,
				KeepBoolean:  true,
				KeepNull:     true,
			},
			SQLExecPlan:          defaultSQLPlanObfuscateSettings,
			SQLExecPlanNormalize: defaultSQLPlanNormalizeSettings,
		})
	})
	return obfuscator
}

// obfuscateSQL obfuscates & normalizes the provided SQL query.
func obfuscateSQL(rawQuery string) (string, error) {
	obfuscatedQuery, err := lazyInitObfuscator().ObfuscateSQLString(rawQuery)
	if err != nil {
		return "", err
	}
	return obfuscatedQuery.Query, nil
}

// obfuscateSQLExecPlan obfuscates the provided json query execution plan.
func obfuscateSQLExecPlan(rawPlan string) (string, error) {
	return lazyInitObfuscator().ObfuscateSQLExecPlan(rawPlan, true)
}

// Ending source(Apache 2.0): https://github.com/DataDog/datadog-agent/blob/main/pkg/collector/python/datadog_agent.go
