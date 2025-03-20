// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
)

const (
	deletionDelay              = 2 * time.Minute
	jitterKubernetesAPISeconds = 10
)

type KubernetesMetadata struct {
	logger               *zap.Logger
	config               *Config
	ready                atomic.Bool
	safeStopCh           *k8sclient.SafeChannel
	mu                   sync.Mutex
	endpointSliceWatcher *k8sclient.EndpointSliceWatcher
}

var _ extension.Extension = (*KubernetesMetadata)(nil)

func jitterSleep(seconds int) {
	jitter := time.Duration(rand.Intn(seconds)) * time.Second // nolint:gosec
	time.Sleep(jitter)
}

func (e *KubernetesMetadata) Start(_ context.Context, _ component.Host) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Debug("Starting k8smetadata extension...")

	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		e.logger.Error("Failed to create config", zap.Error(err))
	}
	e.logger.Debug("Kubernetes config built successfully")

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		e.logger.Error("Failed to create kubernetes client", zap.Error(err))
	}
	e.logger.Debug("Kubernetes clientset created successfully")

	// jitter calls to the kubernetes api (a precaution to prevent overloading api server)
	jitterSleep(jitterKubernetesAPISeconds)

	timedDeleter := &k8sclient.TimedDeleter{Delay: deletionDelay}
	sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 0)

	e.endpointSliceWatcher = k8sclient.NewEndpointSliceWatcher(e.logger, sharedInformerFactory, timedDeleter)
	e.safeStopCh = &k8sclient.SafeChannel{Ch: make(chan struct{}), Closed: false}

	e.logger.Debug("Starting EndpointSliceWatcher Run()")
	e.endpointSliceWatcher.Run(e.safeStopCh.Ch)

	e.logger.Debug("Waiting for EndpointSlice cache to sync...")
	e.endpointSliceWatcher.WaitForCacheSync(e.safeStopCh.Ch)

	e.logger.Debug("EndpointSlice cache synced, extension fully started")
	e.ready.Store(true)

	return nil
}

func (e *KubernetesMetadata) Shutdown(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.safeStopCh.Close()
	return nil
}

func (e *KubernetesMetadata) GetPodMetadata(ip string) k8sclient.PodMetadata {
	if ip == "" {
		e.logger.Debug("GetPodMetadata: no IP provided")
		return k8sclient.PodMetadata{}
	}
	pm, ok := e.endpointSliceWatcher.IPToPodMetadata.Load(ip)
	if !ok {
		e.logger.Debug("GetPodMetadata: no mapping found for IP")
		return k8sclient.PodMetadata{}
	}
	metadata := pm.(k8sclient.PodMetadata)
	e.logger.Debug("GetPodMetadata: found metadata")
	return metadata
}
