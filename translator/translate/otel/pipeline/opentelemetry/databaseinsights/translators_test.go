// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestNewTranslators_MissingKey(t *testing.T) {
	assert.Equal(t, 0, NewTranslators(nil).Len())
	assert.Equal(t, 0, NewTranslators(confmap.NewFromStringMap(map[string]interface{}{})).Len())
}

func TestNewTranslators_SingleInstanceWithLogs(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"database_insights": map[string]interface{}{
					"postgresql": []interface{}{
						map[string]interface{}{
							"endpoint":      "localhost:5432",
							"username":      "cw_monitor",
							"password_file": "/etc/.pgpass",
							"instance_name": "my-db",
							"logs": map[string]interface{}{
								"file_path": "/var/log/postgresql/postgresql.log",
							},
						},
					},
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	assert.Equal(t, 4, translators.Len())
}

func TestNewTranslators_SingleInstanceNoLogs(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"database_insights": map[string]interface{}{
					"postgresql": []interface{}{
						map[string]interface{}{
							"endpoint":      "db.example.com:5432",
							"username":      "cw_monitor",
							"password_file": "/etc/.pgpass",
							"instance_name": "remote-db",
						},
					},
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	assert.Equal(t, 3, translators.Len())
}

func TestNewTranslators_MultiInstance(t *testing.T) {
	cfg := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"database_insights": map[string]interface{}{
					"postgresql": []interface{}{
						map[string]interface{}{
							"endpoint":      "localhost:5432",
							"username":      "cw_monitor",
							"password_file": "/etc/.pgpass",
							"instance_name": "db1",
							"logs": map[string]interface{}{
								"file_path": "/var/log/pg1.log",
							},
						},
						map[string]interface{}{
							"endpoint":      "db2.example.com:5432",
							"username":      "monitor",
							"password_file": "/etc/.pgpass2",
							"instance_name": "db2",
						},
					},
				},
			},
		},
	})
	translators := NewTranslators(cfg)
	assert.Equal(t, 7, translators.Len()) // 4 + 3

	var ids []string
	for _, k := range translators.Keys() {
		ids = append(ids, k.String())
	}
	expected := []string{
		"metrics/dbi_postgresql_0",
		"logs/dbi_postgresql_0",
		"logs/dbi_postgresql_rawevents_0",
		"logs/dbi_postgresql_serverlogs_0",
		"metrics/dbi_postgresql_1",
		"logs/dbi_postgresql_1",
		"logs/dbi_postgresql_rawevents_1",
	}
	assert.ElementsMatch(t, expected, ids)
}

func TestIsLocalhostEndpoint(t *testing.T) {
	assert.True(t, isLocalhostEndpoint("localhost:5432"))
	assert.True(t, isLocalhostEndpoint("127.0.0.1:5432"))
	assert.False(t, isLocalhostEndpoint("db.example.com:5432"))
	assert.False(t, isLocalhostEndpoint(""))
}
