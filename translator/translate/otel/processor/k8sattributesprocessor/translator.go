// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sattributesprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	name = "k8sattributesprocessor"
)

type translator struct {
	name                string
	telemetrySectionKey string
	factory             processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return &translator{
		factory: k8sattributesprocessor.NewFactory(),
	}
}

func NewTranslatorWithNameAndSection(name string, telemetrySectionKey string) common.Translator[component.Config] {
	return &translator{name, telemetrySectionKey, k8sattributesprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*k8sattributesprocessor.Config)
	// TODO: make metadata configurable
	cfg.Extract.Metadata = []string{
		conventions.AttributeK8SNamespaceName,
		conventions.AttributeK8SNodeName,
		conventions.AttributeK8SDeploymentName,
		conventions.AttributeK8SReplicaSetName,
		conventions.AttributeK8SDaemonSetName,
		conventions.AttributeK8SStatefulSetName,
	}
	return cfg, nil
}
