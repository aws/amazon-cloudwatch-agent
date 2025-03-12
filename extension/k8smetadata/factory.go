// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	TypeStr, _            = component.NewType("k8smetadata")
	kubernetesMetadataExt *kubernetesMetadata
	mutex                 sync.RWMutex
)

func GetKubernetesMetadata() *kubernetesMetadata {
	mutex.RLock()
	defer mutex.RUnlock()
	if kubernetesMetadataExt != nil && kubernetesMetadataExt.ready.Load() {
		return kubernetesMetadataExt
	}
	return nil
}

func NewFactory() extension.Factory {
	return extension.NewFactory(
		TypeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelAlpha,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(_ context.Context, settings extension.Settings, cfg component.Config) (extension.Extension, error) {
	mutex.Lock()
	defer mutex.Unlock()
	kubernetesMetadataExt = &kubernetesMetadata{
		logger: settings.Logger,
		config: cfg.(*Config),
	}
	return kubernetesMetadataExt, nil
}
