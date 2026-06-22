// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sattributesprocessor

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a k8sattributes processor that enriches telemetry with
// K8s pod metadata (pod name, namespace, node, workload owners).
// Only intended for use when running in a K8s environment.
func NewTranslator(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: k8sattributesprocessor.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig()

	cfgMap := map[string]interface{}{
		"auth_type":   "serviceAccount",
		"passthrough": false,
		"filter": map[string]interface{}{
			"node_from_env_var": "K8S_NODE_NAME",
		},
		"extract": map[string]interface{}{
			"metadata": []string{
				"k8s.namespace.name",
				"k8s.deployment.name",
				"k8s.replicaset.name",
				"k8s.statefulset.name",
				"k8s.daemonset.name",
				"k8s.job.name",
				"k8s.cronjob.name",
				"k8s.node.name",
				"k8s.pod.name",
				"k8s.pod.uid",
				"k8s.pod.start_time",
				"k8s.container.name",
			},
			"annotations": []map[string]interface{}{
				{"tag_name": "resource.opentelemetry.io/service.name", "key": "resource.opentelemetry.io/service.name", "from": "pod"},
				{"tag_name": "resource.opentelemetry.io/service.namespace", "key": "resource.opentelemetry.io/service.namespace", "from": "pod"},
				{"tag_name": "resource.opentelemetry.io/service.instance.id", "key": "resource.opentelemetry.io/service.instance.id", "from": "pod"},
				{"tag_name": "resource.opentelemetry.io/service.version", "key": "resource.opentelemetry.io/service.version", "from": "pod"},
			},
			"labels": []map[string]interface{}{
				{"tag_name": "app.kubernetes.io/instance", "key": "app.kubernetes.io/instance", "from": "pod"},
				{"tag_name": "app.kubernetes.io/name", "key": "app.kubernetes.io/name", "from": "pod"},
				{"tag_name": "app.kubernetes.io/version", "key": "app.kubernetes.io/version", "from": "pod"},
			},
		},
		"exclude": map[string]interface{}{
			"pods": []interface{}{},
		},
	}

	if err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to configure k8sattributes processor: %w", err)
	}

	return cfg, nil
}
