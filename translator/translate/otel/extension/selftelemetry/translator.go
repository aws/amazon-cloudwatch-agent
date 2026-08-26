// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package selftelemetry

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/extension/selftelemetry"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	// SectionKey is the agent sub-section that turns the endpoint on.
	SectionKey = common.SelfTelemetryKey

	// Host is fixed to loopback. The endpoint exists for a prometheus receiver on the same node to
	// scrape, so it never needs to be reachable from anywhere else and offers no bind address knob.
	Host = "127.0.0.1"
	// DefaultPort follows the collector's own service::telemetry default.
	DefaultPort = 8888

	// NodeNameLabel matches the existing Container Insights CloudWatch dimension, so one alarm or
	// dashboard dimension works across ECI, OCI, and self telemetry alike.
	NodeNameLabel = "NodeName"
	// NodeNameEnvRef is expanded per pod by the config provider from the operator-injected env var.
	NodeNameEnvRef = "${env:K8S_NODE_NAME}"

	enabledKey = "enabled"
	portKey    = "port"
)

type translator struct {
	name    string
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{
		factory: selftelemetry.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	return t.factory.CreateDefaultConfig().(*selftelemetry.Config), nil
}

func configKey(key string) string {
	return common.ConfigKey(common.AgentKey, SectionKey, key)
}

// Enabled reports whether agent::self_telemetry turns the endpoint on.
func Enabled(conf *confmap.Conf) bool {
	enabled, _ := common.GetBool(conf, configKey(enabledKey))
	return enabled
}

// Port returns the loopback port the collector's metrics reader should bind.
func Port(conf *confmap.Conf) int {
	if v, ok := common.GetNumber(conf, configKey(portKey)); ok && v > 0 {
		return int(v)
	}
	return DefaultPort
}
