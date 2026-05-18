// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

func validateJSON(t *testing.T, jsonStr string) *gojsonschema.Result {
	t.Helper()
	var input map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &input))
	schemaLoader := gojsonschema.NewStringLoader(GetJsonSchema())
	jsonLoader := gojsonschema.NewGoLoader(input)
	result, err := gojsonschema.Validate(schemaLoader, jsonLoader)
	require.NoError(t, err)
	return result
}

func TestSyslogSchema_SingleListener(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/syslog",
					"log_stream_name": "stream"
				}
			}
		}
	}`)
	assert.True(t, result.Valid(), "single syslog listener should be valid: %v", result.Errors())
}

func TestSyslogSchema_MultipleListeners(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": [
					{
						"listen_address": "tcp://0.0.0.0:514",
						"log_group_name": "/test/tcp"
					},
					{
						"listen_address": "udp://0.0.0.0:514",
						"log_group_name": "/test/udp"
					}
				]
			}
		}
	}`)
	assert.True(t, result.Valid(), "multiple syslog listeners should be valid: %v", result.Errors())
}

func TestSyslogSchema_WithRouting(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"routing": [
						{
							"match": {"hostname": "web-*"},
							"log_group_name": "/test/web"
						}
					]
				}
			}
		}
	}`)
	assert.True(t, result.Valid(), "syslog with routing should be valid: %v", result.Errors())
}

func TestSyslogSchema_WithFilters(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"filters": [
						{"type": "exclude", "expression": "healthcheck"},
						{"type": "include", "expression": "error|warn"}
					]
				}
			}
		}
	}`)
	assert.True(t, result.Valid(), "syslog with filters should be valid: %v", result.Errors())
}

func TestSyslogSchema_WithTLS(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/tls",
					"tls": {
						"cert_file": "/etc/ssl/cert.pem",
						"key_file": "/etc/ssl/key.pem",
						"ca_file": "/etc/ssl/ca.pem",
						"min_version": "1.3"
					}
				}
			}
		}
	}`)
	assert.True(t, result.Valid(), "syslog with TLS should be valid: %v", result.Errors())
}

func TestSyslogSchema_WithProtocol(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "udp://0.0.0.0:514",
					"protocol": "rfc3164",
					"log_group_name": "/test/bsd"
				}
			}
		}
	}`)
	assert.True(t, result.Valid(), "syslog with protocol should be valid: %v", result.Errors())
}

func TestSyslogSchema_WithRetention(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"retention_in_days": 30,
					"routing": [
						{
							"match": {"facility": 4},
							"log_group_name": "/test/auth",
							"retention_in_days": 7
						}
					]
				}
			}
		}
	}`)
	assert.True(t, result.Valid(), "syslog with retention should be valid: %v", result.Errors())
}

func TestSyslogSchema_FullConfig(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": [
					{
						"listen_address": "tcp://0.0.0.0:514",
						"log_group_name": "/test/default",
						"log_stream_name": "default",
						"retention_in_days": 30,
						"filters": [{"type": "exclude", "expression": "healthcheck"}],
						"routing": [
							{
								"match": {"hostname": "web-*", "facility": 1},
								"log_group_name": "/test/web",
								"log_stream_name": "web",
								"retention_in_days": 7,
								"filters": [{"type": "include", "expression": "error|warn"}]
							}
						]
					},
					{
						"listen_address": "udp://0.0.0.0:514",
						"protocol": "rfc3164",
						"log_group_name": "/test/udp"
					},
					{
						"listen_address": "tcp://0.0.0.0:1514",
						"log_group_name": "/test/tls",
						"tls": {"cert_file": "/cert.pem", "key_file": "/key.pem"}
					}
				]
			}
		}
	}`)
	assert.True(t, result.Valid(), "full syslog config should be valid: %v", result.Errors())
}

func TestSyslogSchema_InvalidTopLevelKey(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default"
				}
			},
			"invalid_key": true
		}
	}`)
	assert.False(t, result.Valid(), "invalid key under logs should fail validation")
}

func TestSyslogSchema_MissingListenAddress(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"log_group_name": "/test/default"
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "missing listen_address should fail")
}

func TestSyslogSchema_MissingLogGroupName(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514"
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "missing log_group_name should fail")
}

func TestSyslogSchema_InvalidProtocol(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"protocol": "rfc9999"
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "invalid protocol should fail")
}

func TestSyslogSchema_InvalidTLSMinVersion(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"tls": { "min_version": "2.0" }
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "invalid TLS min_version should fail")
}

func TestSyslogSchema_InvalidRetention(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"retention_in_days": 42
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "invalid retention_in_days value should fail")
}

func TestSyslogSchema_InvalidFacility(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"routing": [{
						"match": { "facility": 99 },
						"log_group_name": "/test/bad"
					}]
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "facility > 23 should fail")
}

func TestSyslogSchema_RoutingMissingMatch(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"routing": [{
						"log_group_name": "/test/norule"
					}]
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "routing rule missing match should fail")
}

func TestSyslogSchema_RoutingMissingLogGroup(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"routing": [{
						"match": { "hostname": "web-*" }
					}]
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "routing rule missing log_group_name should fail")
}

func TestSyslogSchema_RoutingEmptyMatch(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"routing": [{
						"match": {},
						"log_group_name": "/test/empty"
					}]
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "routing rule with empty match should fail")
}

func TestSyslogSchema_UnknownListenerField(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"bogus_field": true
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "unknown field on listener should fail")
}

func TestSyslogSchema_UnknownTLSField(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"tls": { "cipher_suites": ["TLS_AES_128"] }
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "unknown TLS field should fail")
}

func TestSyslogSchema_WithClientCAFile(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:6514",
					"log_group_name": "/test/mtls",
					"tls": {
						"cert_file": "/etc/ssl/cert.pem",
						"key_file": "/etc/ssl/key.pem",
						"client_ca_file": "/etc/ssl/client-ca.pem",
						"min_version": "1.2"
					}
				}
			}
		}
	}`)
	assert.True(t, result.Valid(), "syslog with client_ca_file should be valid: %v", result.Errors())
}

func TestSyslogSchema_UnknownMatchField(t *testing.T) {
	result := validateJSON(t, `{
		"logs": {
			"logs_collected": {
				"syslog": {
					"listen_address": "tcp://0.0.0.0:514",
					"log_group_name": "/test/default",
					"routing": [{
						"match": { "severity": 3 },
						"log_group_name": "/test/bad"
					}]
				}
			}
		}
	}`)
	assert.False(t, result.Valid(), "unknown match field should fail")
}
