// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"context"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type KubernetesMetadata struct {
	logger *zap.Logger
	config *Config

	mu sync.Mutex

	clientset             kubernetes.Interface
	sharedInformerFactory cache.SharedInformer

	ipToPodMetadata *sync.Map

	endpointSliceWatcher *k8sclient.EndpointSliceWatcher
}

var _ extension.Extension = (*KubernetesMetadata)(nil)

func (e *KubernetesMetadata) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (e *KubernetesMetadata) Shutdown(_ context.Context) error {
	return nil
}
