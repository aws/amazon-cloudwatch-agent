// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package healthcheck

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type healthCheckTranslator struct {
	name string
}

var _ common.Translator[component.Config, component.ID] = (*healthCheckTranslator)(nil)

func NewHealthCheckTranslator() common.Translator[component.Config, component.ID] {
	return &healthCheckTranslator{name: "health_check"}
}

func (t *healthCheckTranslator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType("health_check"), t.name)
}

func (t *healthCheckTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := &struct {
		Endpoint string `mapstructure:"endpoint"`
		Path     string `mapstructure:"path"`
	}{
		Endpoint: "0.0.0.0:13133",
		Path:     "/",
	}

	return cfg, nil
}
