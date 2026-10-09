// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslatorID(t *testing.T) {
	tt := NewTranslator("tcp_0_0_0_0_514", "tcp://0.0.0.0:514", "", nil)
	assert.Equal(t, "syslog/tcp_0_0_0_0_514", tt.ID().String())
}

func TestTranslateTCP(t *testing.T) {
	tt := NewTranslator("tcp_test", "tcp://0.0.0.0:514", "", nil)
	cfg, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestTranslateUDP(t *testing.T) {
	tt := NewTranslator("udp_test", "udp://0.0.0.0:514", "", nil)
	cfg, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestTranslateInvalidAddress(t *testing.T) {
	tt := NewTranslator("bad", "invalid-address", "", nil)
	_, err := tt.Translate(confmap.New())
	assert.Error(t, err)
}

func TestTranslateUnsupportedProtocol(t *testing.T) {
	tt := NewTranslator("bad", "ws://0.0.0.0:514", "", nil)
	_, err := tt.Translate(confmap.New())
	assert.Error(t, err)
}

func TestTranslateRFC3164(t *testing.T) {
	tt := NewTranslator("tcp_test", "tcp://0.0.0.0:514", "rfc3164", nil)
	cfg, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestTranslateDefaultProtocol(t *testing.T) {
	tt := NewTranslator("tcp_test", "tcp://0.0.0.0:514", "", nil)
	cfg, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestTranslateTCPWithTLS(t *testing.T) {
	tlsCfg := map[string]any{
		"cert_file":   "/path/to/cert.pem",
		"key_file":    "/path/to/key.pem",
		"ca_file":     "/path/to/ca.pem",
		"min_version": "1.3",
	}
	tt := NewTranslator("tls_test", "tcp://0.0.0.0:6514", "", tlsCfg)
	cfg, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestTranslateTCPWithClientCAFile(t *testing.T) {
	tlsCfg := map[string]any{
		"cert_file":      "/path/to/cert.pem",
		"key_file":       "/path/to/key.pem",
		"client_ca_file": "/path/to/client-ca.pem",
	}
	tt := NewTranslator("mtls_test", "tcp://0.0.0.0:6514", "", tlsCfg)
	cfg, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	out := confmap.New()
	require.NoError(t, out.Marshal(cfg))
	assert.Equal(t, "/path/to/client-ca.pem", out.Get("tcp::tls::client_ca_file"))
	assert.Equal(t, "/path/to/cert.pem", out.Get("tcp::tls::cert_file"))
	assert.Equal(t, "/path/to/key.pem", out.Get("tcp::tls::key_file"))
}

func TestTranslateUDPIgnoresTLS(t *testing.T) {
	tlsCfg := map[string]any{
		"cert_file": "/path/to/cert.pem",
		"key_file":  "/path/to/key.pem",
	}
	tt := NewTranslator("udp_tls_test", "udp://0.0.0.0:514", "", tlsCfg)
	cfg, err := tt.Translate(confmap.New())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestParseListenAddress(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		protocol string
		address  string
		wantErr  bool
	}{
		{"TCP", "tcp://0.0.0.0:514", "tcp", "0.0.0.0:514", false},
		{"UDP", "udp://127.0.0.1:6515", "udp", "127.0.0.1:6515", false},
		{"MissingProtocol", "0.0.0.0:514", "", "", true},
		{"UnsupportedProtocol", "ws://0.0.0.0:514", "", "", true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			protocol, address, err := parseListenAddress(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.protocol, protocol)
				assert.Equal(t, tc.address, address)
			}
		})
	}
}
