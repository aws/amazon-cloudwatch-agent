// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"context"
	"math/rand"
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
	endpointSliceWatcher *k8sclient.EndpointSliceWatcher
	serviceWatcher       *k8sclient.ServiceWatcher
}

var _ extension.Extension = (*KubernetesMetadata)(nil)

func jitterSleep(seconds int) {
	jitter := time.Duration(rand.Intn(seconds)) * time.Second // nolint:gosec
	time.Sleep(jitter)
}

func (e *KubernetesMetadata) Start(_ context.Context, _ component.Host) error {
	e.logger.Debug("Starting k8smetadata extension...")

	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		e.logger.Error("Failed to create config", zap.Error(err))
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		e.logger.Error("Failed to create kubernetes client", zap.Error(err))
	}

	// jitter calls to the kubernetes api (a precaution to prevent overloading api server)
	jitterSleep(jitterKubernetesAPISeconds)

	timedDeleter := &k8sclient.TimedDeleter{Delay: deletionDelay}
	sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 0)
	e.safeStopCh = &k8sclient.SafeChannel{Ch: make(chan struct{}), Closed: false}

	for _, obj := range e.config.Objects {
		switch obj {
		case "endpointslices":
			e.endpointSliceWatcher = k8sclient.NewEndpointSliceWatcher(e.logger, sharedInformerFactory, timedDeleter)
			e.endpointSliceWatcher.Run(e.safeStopCh.Ch)
			e.endpointSliceWatcher.WaitForCacheSync(e.safeStopCh.Ch)
			e.logger.Debug("EndpointSlice cache synced")
		case "services":
			e.serviceWatcher = k8sclient.NewServiceWatcher(e.logger, sharedInformerFactory, timedDeleter)
			e.serviceWatcher.Run(e.safeStopCh.Ch)
			e.serviceWatcher.WaitForCacheSync(e.safeStopCh.Ch)
			e.logger.Debug("Service cache synced")
		}
	}

	e.logger.Debug("Cache synced, extension fully started")
	e.ready.Store(true)

	return nil
}

func (e *KubernetesMetadata) Shutdown(_ context.Context) error {
	if e.safeStopCh != nil {
		e.safeStopCh.Close()
	}
	return nil
}

func (e *KubernetesMetadata) GetPodMetadataFromPodIP(ip string) k8sclient.PodMetadata {
	if e.endpointSliceWatcher == nil {
		e.logger.Debug("GetPodMetadataFromPodIP: endpointslices not enabled in config")
		return k8sclient.PodMetadata{}
	}
	if ip == "" {
		e.logger.Debug("GetPodMetadataFromPodIP: no IP provided")
		return k8sclient.PodMetadata{}
	}
	pm, ok := e.endpointSliceWatcher.GetIPToPodMetadata().Load(ip)
	if !ok {
		e.logger.Debug("GetPodMetadataFromPodIP: no mapping found for IP", zap.String("ip", ip))
		return k8sclient.PodMetadata{}
	}
	metadata := pm.(k8sclient.PodMetadata)
	e.logger.Debug("GetPodMetadataFromPodIP: found metadata",
		zap.String("ip", ip),
		zap.String("workload", metadata.Workload),
		zap.String("namespace", metadata.Namespace),
		zap.String("node", metadata.Node),
	)
	return metadata
}

func (e *KubernetesMetadata) GetPodMetadataFromServiceAndNamespace(svcAndNS string) k8sclient.PodMetadata {
	if e.endpointSliceWatcher == nil {
		e.logger.Debug("GetPodMetadataFromServiceAndNamespace: endpointslices not enabled in config")
		return k8sclient.PodMetadata{}
	}
	if svcAndNS == "" {
		e.logger.Debug("GetPodMetadataFromServiceAndNamespace: no service@namespace provided")
		return k8sclient.PodMetadata{}
	}
	pm, ok := e.endpointSliceWatcher.GetServiceNamespaceToPodMetadata().Load(svcAndNS)
	if !ok {
		e.logger.Debug("GetPodMetadataFromServiceAndNamespace: no mapping found", zap.String("svcAndNS", svcAndNS))
		return k8sclient.PodMetadata{}
	}
	metadata := pm.(k8sclient.PodMetadata)
	e.logger.Debug("GetPodMetadataFromServiceAndNamespace: found metadata",
		zap.String("serviceNameAndNamespace", svcAndNS),
		zap.String("workload", metadata.Workload),
		zap.String("node", metadata.Node),
	)
	return metadata
}

func (e *KubernetesMetadata) GetServiceAndNamespaceFromClusterIP(ip string) string {
	if e.serviceWatcher == nil {
		e.logger.Debug("GetServiceAndNamespaceFromClusterIP: services not enabled in config")
		return ""
	}
	if ip == "" {
		e.logger.Debug("GetServiceAndNamespaceFromClusterIP: no IP provided")
		return ""
	}
	svcAndNS, ok := e.serviceWatcher.GetIPToServiceAndNamespace().Load(ip)
	if !ok {
		e.logger.Debug("GetServiceAndNamespaceFromClusterIP: no mapping found", zap.String("ip", ip))
		return ""
	}
	svcAndNSString := svcAndNS.(string)
	e.logger.Debug("GetServiceAndNamespaceFromClusterIP: found metadata",
		zap.String("ip", ip),
		zap.String("svcAndNS", svcAndNSString),
	)
	return svcAndNSString
}
