// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name          string
	listenAddress string
	protocol      string
	tlsConfig     map[string]any
	factory       receiver.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a new syslog receiver translator.
func NewTranslator(name string, listenAddress string, protocol string, tlsConfig map[string]any) common.ComponentTranslator {
	if protocol == "" {
		protocol = "rfc5424"
	}
	return &translator{
		name:          name,
		listenAddress: listenAddress,
		protocol:      protocol,
		tlsConfig:     tlsConfig,
		factory:       syslogreceiver.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a syslog receiver config based on the listen address.
// The listenAddress is expected in the format "tcp://host:port" or "udp://host:port".
func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	protocol, address, err := parseListenAddress(t.listenAddress)
	if err != nil {
		return nil, fmt.Errorf("syslog translator %s: %w", t.name, err)
	}

	cfgMap := map[string]any{
		protocol: map[string]any{
			"listen_address": address,
		},
		"protocol": t.protocol,
	}

	// TLS only applies to TCP listeners
	if protocol == "tcp" && len(t.tlsConfig) > 0 {
		cfgMap[protocol].(map[string]any)["tls"] = t.tlsConfig
	}

	cfg := t.factory.CreateDefaultConfig()
	if err := confmap.NewFromStringMap(cfgMap).Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("syslog translator %s: unable to unmarshal config: %w", t.name, err)
	}
	return cfg, nil
}

// parseListenAddress parses "tcp://host:port" or "udp://host:port" into
// the transport protocol and host:port address.
func parseListenAddress(listenAddress string) (string, string, error) {
	parts := strings.SplitN(listenAddress, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid listen address %q, expected protocol://host:port", listenAddress)
	}
	protocol := strings.ToLower(parts[0])
	if protocol != "tcp" && protocol != "udp" {
		return "", "", fmt.Errorf("unsupported protocol %q, expected tcp or udp", protocol)
	}
	return protocol, parts[1], nil
}
