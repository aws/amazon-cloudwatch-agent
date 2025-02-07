// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	attr "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
)

const (
	// Deletion delay adjustment:
	// Previously, EKS resolver would instantly remove the IP to Service mapping when a pod was destroyed.
	// This posed a problem because:
	//   1. Metric data is aggregated and emitted every 1 minute.
	//   2. If this aggregated metric data, which contains the IP of the now-destroyed pod, arrives
	//      at the EKS resolver after the IP records have already been deleted, the metric can't be processed correctly.
	//
	// To mitigate this issue, we've introduced a 2-minute deletion delay. This ensures that any
	// metric data that arrives within those 2 minutes, containing the old IP, will still get mapped correctly to a service.
	deletionDelay = 2 * time.Minute

	jitterKubernetesAPISeconds = 10
)

type kubernetesResolver struct {
	logger       *zap.Logger
	clientset    kubernetes.Interface
	clusterName  string
	platformCode string
	// if ListPod api is used, the following maps are needed
	ipToPod                      *sync.Map
	podToWorkloadAndNamespace    *sync.Map
	workloadAndNamespaceToLabels *sync.Map
	workloadPodCount             map[string]int

	// if ListEndpointSlice api is used, the following maps are needed
	ipToWorkloadAndNamespace *sync.Map

	// if ListService api is used, the following maps are needed
	ipToServiceAndNamespace        *sync.Map
	serviceAndNamespaceToSelectors *sync.Map

	// if ListPod and ListService apis are used, the serviceToWorkload map is computed by ServiceToWorkloadMapper
	// from serviceAndNamespaceToSelectors and workloadAndNamespaceToLabels every 1 min
	// if ListEndpointSlice is used, we can get serviceToWorkload directly from endpointSlice watcher
	serviceToWorkload *sync.Map //

	safeStopCh *safeChannel // trace and metric processors share the same kubernetesResolver and might close the same channel separately
	useListPod bool
}

var (
	once     sync.Once
	instance *kubernetesResolver
)

func jitterSleep(seconds int) {
	jitter := time.Duration(rand.Intn(seconds)) * time.Second // nolint:gosec
	time.Sleep(jitter)
}

func getKubernetesResolver(platformCode, clusterName string, logger *zap.Logger) subResolver {
	once.Do(func() {
		config, err := clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			logger.Fatal("Failed to create config", zap.Error(err))
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			logger.Fatal("Failed to create kubernetes client", zap.Error(err))
		}

		// jitter calls to the kubernetes api
		jitterSleep(jitterKubernetesAPISeconds)

		useListPod := (os.Getenv("USE_LIST_POD") == "true")

		if useListPod {
			sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 0)
			timedDeleter := &TimedDeleter{Delay: deletionDelay}

			poWatcher := newPodWatcher(logger, sharedInformerFactory, timedDeleter)
			svcWatcher := newServiceWatcher(logger, sharedInformerFactory, timedDeleter)

			safeStopCh := &safeChannel{ch: make(chan struct{}), closed: false}
			// initialize the pod and service watchers for the cluster
			poWatcher.run(safeStopCh.ch)
			svcWatcher.Run(safeStopCh.ch)
			// wait for caches to sync (for once) so that clients knows about the pods and services in the cluster
			poWatcher.waitForCacheSync(safeStopCh.ch)
			svcWatcher.waitForCacheSync(safeStopCh.ch)

			serviceToWorkload := &sync.Map{}
			svcToWorkloadMapper := newServiceToWorkloadMapper(svcWatcher.serviceAndNamespaceToSelectors, poWatcher.workloadAndNamespaceToLabels, serviceToWorkload, logger, timedDeleter)
			svcToWorkloadMapper.Start(safeStopCh.ch)

			instance = &kubernetesResolver{
				logger:                         logger,
				clientset:                      clientset,
				clusterName:                    clusterName,
				platformCode:                   platformCode,
				ipToServiceAndNamespace:        svcWatcher.ipToServiceAndNamespace,
				serviceAndNamespaceToSelectors: svcWatcher.serviceAndNamespaceToSelectors,
				ipToPod:                        poWatcher.ipToPod,
				podToWorkloadAndNamespace:      poWatcher.podToWorkloadAndNamespace,
				workloadAndNamespaceToLabels:   poWatcher.workloadAndNamespaceToLabels,
				serviceToWorkload:              serviceToWorkload,
				workloadPodCount:               poWatcher.workloadPodCount,
				ipToWorkloadAndNamespace:       nil,
				safeStopCh:                     safeStopCh,
				useListPod:                     useListPod,
			}
		} else {
			sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 0)
			timedDeleter := &TimedDeleter{Delay: deletionDelay}

			svcWatcher := newServiceWatcher(logger, sharedInformerFactory, timedDeleter)
			endptSliceWatcher := newEndpointSliceWatcher(logger, sharedInformerFactory, timedDeleter)

			safeStopCh := &safeChannel{ch: make(chan struct{}), closed: false}
			// initialize the pod and service watchers for the cluster
			svcWatcher.Run(safeStopCh.ch)
			endptSliceWatcher.Run(safeStopCh.ch)
			// wait for caches to sync (for once) so that clients knows about the pods and services in the cluster
			svcWatcher.waitForCacheSync(safeStopCh.ch)
			endptSliceWatcher.waitForCacheSync(safeStopCh.ch)

			instance = &kubernetesResolver{
				logger:                       logger,
				clientset:                    clientset,
				clusterName:                  clusterName,
				platformCode:                 platformCode,
				ipToWorkloadAndNamespace:     endptSliceWatcher.ipToWorkload, // endpointSlice provides pod IP → workload mapping
				ipToPod:                      nil,
				podToWorkloadAndNamespace:    nil,
				workloadAndNamespaceToLabels: nil,
				workloadPodCount:             nil,
				ipToServiceAndNamespace:      svcWatcher.ipToServiceAndNamespace,
				serviceToWorkload:            endptSliceWatcher.serviceToWorkload, // endpointSlice also provides service → workload mapping
				safeStopCh:                   safeStopCh,
				useListPod:                   useListPod,
			}

		}
	})

	return instance
}

func (e *kubernetesResolver) Stop(_ context.Context) error {
	e.safeStopCh.Close()
	return nil
}

// add a method to kubernetesResolver
func (e *kubernetesResolver) getWorkloadAndNamespaceByIP(ip string) (string, string, error) {
	var workload, namespace string

	if e.useListPod {
		// use results from pod watcher
		if podKey, ok := e.ipToPod.Load(ip); ok {
			pod := podKey.(string)
			if workloadKey, ok := e.podToWorkloadAndNamespace.Load(pod); ok {
				workload, namespace = extractResourceAndNamespace(workloadKey.(string))
				return workload, namespace, nil
			}
		}
	} else {
		// use results from endpoint slice watcher
		if workloadKey, ok := e.ipToWorkloadAndNamespace.Load(ip); ok {
			workload, namespace = extractResourceAndNamespace(workloadKey.(string))
			return workload, namespace, nil
		}
	}

	if serviceKey, ok := e.ipToServiceAndNamespace.Load(ip); ok {
		serviceAndNamespace := serviceKey.(string)
		if workloadKey, ok := e.serviceToWorkload.Load(serviceAndNamespace); ok {
			workload, namespace = extractResourceAndNamespace(workloadKey.(string))
			return workload, namespace, nil
		}
	}
	return "", "", errors.New("no kubernetes workload found for ip: " + ip)
}

func (e *kubernetesResolver) Process(attributes, resourceAttributes pcommon.Map) error {
	var namespace string
	if value, ok := attributes.Get(attr.AWSRemoteService); ok {
		valueStr := value.AsString()
		ipStr := ""
		if ip, _, ok := extractIPPort(valueStr); ok {
			if workload, ns, err := e.getWorkloadAndNamespaceByIP(valueStr); err == nil {
				attributes.PutStr(attr.AWSRemoteService, workload)
				namespace = ns
			} else {
				ipStr = ip
			}
		} else if isIP(valueStr) {
			ipStr = valueStr
		}

		if ipStr != "" {
			if workload, ns, err := e.getWorkloadAndNamespaceByIP(ipStr); err == nil {
				attributes.PutStr(attr.AWSRemoteService, workload)
				namespace = ns
			} else {
				e.logger.Debug("failed to Process ip", zap.String("ip", ipStr), zap.Error(err))
			}
		}
	}

	if _, ok := attributes.Get(attr.AWSRemoteEnvironment); !ok {
		if namespace != "" {
			attributes.PutStr(attr.AWSRemoteEnvironment, fmt.Sprintf("%s:%s/%s", e.platformCode, e.clusterName, namespace))
		}
	}

	return nil
}

type kubernetesResourceAttributesResolver struct {
	platformCode string
	clusterName  string
	attributeMap map[string]string
}

func newKubernetesResourceAttributesResolver(platformCode, clusterName string) *kubernetesResourceAttributesResolver {
	return &kubernetesResourceAttributesResolver{
		platformCode: platformCode,
		clusterName:  clusterName,
		attributeMap: DefaultInheritedAttributes,
	}
}
func (h *kubernetesResourceAttributesResolver) Process(attributes, resourceAttributes pcommon.Map) error {
	for attrKey, mappingKey := range h.attributeMap {
		if val, ok := resourceAttributes.Get(attrKey); ok {
			attributes.PutStr(mappingKey, val.AsString())
		}
	}
	if h.platformCode == config.PlatformEKS {
		attributes.PutStr(common.AttributePlatformType, AttributePlatformEKS)
		attributes.PutStr(common.AttributeEKSClusterName, h.clusterName)
	} else {
		attributes.PutStr(common.AttributePlatformType, AttributePlatformK8S)
		attributes.PutStr(common.AttributeK8SClusterName, h.clusterName)
	}
	var namespace string
	if nsAttr, ok := resourceAttributes.Get(semconv.AttributeK8SNamespaceName); ok {
		namespace = nsAttr.Str()
	} else {
		namespace = "UnknownNamespace"
	}

	if val, ok := attributes.Get(attr.AWSLocalEnvironment); !ok {
		env := generateLocalEnvironment(h.platformCode, h.clusterName+"/"+namespace)
		attributes.PutStr(attr.AWSLocalEnvironment, env)
	} else {
		attributes.PutStr(attr.AWSLocalEnvironment, val.Str())
	}

	attributes.PutStr(common.AttributeK8SNamespace, namespace)
	//The application log group in Container Insights is a fixed pattern:
	// "/aws/containerinsights/{Cluster_Name}/application"
	// See https://github.com/aws/amazon-cloudwatch-agent-operator/blob/fe144bb02d7b1930715aa3ea32e57a5ff13406aa/helm/templates/fluent-bit-configmap.yaml#L82
	logGroupName := "/aws/containerinsights/" + h.clusterName + "/application"
	resourceAttributes.PutStr(semconv.AttributeAWSLogGroupNames, logGroupName)

	return nil
}

func (h *kubernetesResourceAttributesResolver) Stop(ctx context.Context) error {
	return nil
}
