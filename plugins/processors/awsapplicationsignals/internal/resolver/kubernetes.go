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

	"github.com/aws/amazon-cloudwatch-agent/extension/k8smetadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
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

	// this is an environmental variable that might deprecate in future
	// when it's "true", we will use list pods API to get ip to workload mapping
	// otherwise, we will use list endpoint slices API instead
	appSignalsUseListPod = "APP_SIGNALS_USE_LIST_POD"
)

type kubernetesResolver struct {
	logger       *zap.Logger
	clientset    kubernetes.Interface
	clusterName  string
	platformCode string

	// If using the extension, no mappings wil be needed
	useExtension bool

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

	safeStopCh *k8sclient.SafeChannel // trace and metric processors share the same kubernetesResolver and might close the same channel separately
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
		// Check environment for "list pods" approach
		useListPod := (os.Getenv(appSignalsUseListPod) == "true")

		if useListPod {
			logger.Info("APP_SIGNALS_USE_LIST_POD=true; setting up Pod & Service watchers, ignoring extension")

			cfg, err := clientcmd.BuildConfigFromFlags("", "")
			if err != nil {
				logger.Fatal("Failed to create config", zap.Error(err))
			}

			clientset, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				logger.Fatal("Failed to create kubernetes client", zap.Error(err))
			}

			jitterSleep(jitterKubernetesAPISeconds)

			sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 0)
			timedDeleter := &k8sclient.TimedDeleter{Delay: deletionDelay}

			poWatcher := newPodWatcher(logger, sharedInformerFactory, timedDeleter)
			svcWatcher := k8sclient.NewServiceWatcher(logger, sharedInformerFactory, timedDeleter)

			safeStopCh := &k8sclient.SafeChannel{Ch: make(chan struct{}), Closed: false}
			// initialize the pod and service watchers for the cluster
			poWatcher.run(safeStopCh.Ch)
			svcWatcher.Run(safeStopCh.Ch)
			// wait for caches to sync (for once) so that clients knows about the pods and services in the cluster
			poWatcher.waitForCacheSync(safeStopCh.Ch)
			svcWatcher.WaitForCacheSync(safeStopCh.Ch)

			serviceToWorkload := &sync.Map{}
			svcToWorkloadMapper := k8sclient.NewServiceToWorkloadMapper(svcWatcher.GetServiceAndNamespaceToSelectors(), poWatcher.workloadAndNamespaceToLabels, serviceToWorkload, logger, timedDeleter)
			svcToWorkloadMapper.Start(safeStopCh.Ch)

			instance = &kubernetesResolver{
				logger:                         logger,
				clientset:                      clientset,
				clusterName:                    clusterName,
				platformCode:                   platformCode,
				useExtension:                   false,
				ipToServiceAndNamespace:        svcWatcher.GetIPToServiceAndNamespace(),
				serviceAndNamespaceToSelectors: svcWatcher.GetServiceAndNamespaceToSelectors(),
				ipToPod:                        poWatcher.ipToPod,
				podToWorkloadAndNamespace:      poWatcher.podToWorkloadAndNamespace,
				workloadAndNamespaceToLabels:   poWatcher.workloadAndNamespaceToLabels,
				serviceToWorkload:              serviceToWorkload,
				workloadPodCount:               poWatcher.workloadPodCount,
				ipToWorkloadAndNamespace:       nil,
				safeStopCh:                     safeStopCh,
				useListPod:                     true,
			}
			return
		}

		// 2) If not using listPod, check if extension is present
		ext := k8smetadata.GetKubernetesMetadata()
		if ext != nil {
			// We skip all watchers (the extension has them).
			logger.Info("k8smetadata extension is present")

			instance = &kubernetesResolver{
				logger:       logger,
				clusterName:  clusterName,
				platformCode: platformCode,
				useExtension: true,
			}
			return
		}

		// 3) Extension is not present, and useListPod is false -> EndpointSlice approach
		logger.Info("k8smetadata extension not found; setting up EndpointSlice watchers")

		cfg, err := clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			logger.Fatal("Failed to create config", zap.Error(err))
		}

		clientset, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			logger.Fatal("Failed to create kubernetes client", zap.Error(err))
		}

		jitterSleep(jitterKubernetesAPISeconds)

		sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 0)

		// For the endpoint slice watcher, we maintain two mappings:
		//   1. ip -> workload
		//   2. service -> workload
		//
		// Scenario:
		//   When a deployment associated with service X has only one pod, the following events occur:
		//     a. A pod terminates (one endpoint terminating). For this event, we add the service -> workload mapping immediately
		//     b. The endpoints become empty (null endpoints). For this event, we remove the service -> workload mapping in a delay way
		//     c. A new pod starts (one endpoint starting). For this event, we add the same service -> workload mapping immediately
		//
		// Problem:
		//   In step (b), a deletion delay (e.g., 2 minutes) is initiated for the mapping with service key X.
		//   Then, in step (c), the mapping for service key X is re-added. Since the new mapping is inserted
		//   before the delay expires, the scheduled deletion from step (b) may erroneously remove the mapping
		//   added in step (c).
		//
		// Root Cause and Resolution:
		//   The issue is caused by deleting the mapping using only the key, without verifying the value.
		//   To fix this, we need to compare both the key and the value before deletion.
		//   That is exactly the purpose of TimedDeleterWithIDCheck.
		TimedDeleterWithIDCheck := &k8sclient.TimedDeleterWithIDCheck{Delay: deletionDelay}
		endptSliceWatcher := k8sclient.NewEndpointSliceWatcher(logger, sharedInformerFactory, TimedDeleterWithIDCheck)

		// for service watcher, we are doing the mapping from IP to service name, it's very rare for an ip to be reused
		// by two services. So we don't face the issue of service -> workload mapping in endpointSliceWatcher.
		// Technically, we can use TimedDeleterWithIDCheck as well but it will involve changing podwatcher with a log of code changes.
		// I don't think it's worthwhile to do it now. We might conside to do it when podwatcher is no longer in use.
		timedDeleter := &k8sclient.TimedDeleter{Delay: deletionDelay}
		svcWatcher := k8sclient.NewServiceWatcher(logger, sharedInformerFactory, timedDeleter)

		safeStopCh := &k8sclient.SafeChannel{Ch: make(chan struct{}), Closed: false}
		// initialize the pod and service watchers for the cluster
		svcWatcher.Run(safeStopCh.Ch)
		endptSliceWatcher.Run(safeStopCh.Ch)
		// wait for caches to sync (for once) so that clients knows about the pods and services in the cluster
		svcWatcher.WaitForCacheSync(safeStopCh.Ch)
		endptSliceWatcher.WaitForCacheSync(safeStopCh.Ch)

		instance = &kubernetesResolver{
			logger:                       logger,
			clientset:                    clientset,
			clusterName:                  clusterName,
			platformCode:                 platformCode,
			ipToWorkloadAndNamespace:     endptSliceWatcher.GetIPToPodMetadata(), // endpointSlice provides pod IP → PodMetadata mapping
			ipToPod:                      nil,
			podToWorkloadAndNamespace:    nil,
			workloadAndNamespaceToLabels: nil,
			workloadPodCount:             nil,
			ipToServiceAndNamespace:      svcWatcher.GetIPToServiceAndNamespace(),
			serviceToWorkload:            endptSliceWatcher.GetServiceNamespaceToPodMetadata(), // endpointSlice also provides service → PodMetadata mapping
			safeStopCh:                   safeStopCh,
			useListPod:                   useListPod,
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
	// If extension is available, rely on that
	if e.useExtension {
		ext := k8smetadata.GetKubernetesMetadata()
		if ext == nil {
			return "", "", errors.New("extension not found (unexpected)")
		}
		pm := ext.GetPodMetadataFromPodIP(ip)
		if pm.Workload != "" {
			return pm.Workload, pm.Namespace, nil
		}

		if svcKeyVal := ext.GetServiceAndNamespaceFromClusterIP(ip); svcKeyVal != "" {
			sm := ext.GetPodMetadataFromServiceAndNamespace(svcKeyVal)
			if sm.Workload != "" {
				return sm.Workload, sm.Namespace, nil
			}
		}
		return "", "", fmt.Errorf("extension could not resolve IP: %s", ip)
	}

	// Otherwise watchers
	if e.useListPod {
		// use results from pod watcher
		if podKey, ok := e.ipToPod.Load(ip); ok {
			pod := podKey.(string)
			if workloadKey, ok := e.podToWorkloadAndNamespace.Load(pod); ok {
				workload, namespace := k8sclient.ExtractResourceAndNamespace(workloadKey.(string))
				return workload, namespace, nil
			}
		}
	} else {
		// use results from endpoint slice watcher
		if pmVal, ok := e.ipToWorkloadAndNamespace.Load(ip); ok {
			pm := pmVal.(k8sclient.PodMetadata)
			return pm.Workload, pm.Namespace, nil
		}
	}

	// Not found in IP->workload, so check IP->service, then service->workload
	if svcKeyVal, ok := e.ipToServiceAndNamespace.Load(ip); ok {
		svcAndNS := svcKeyVal.(string)
		if e.serviceToWorkload != nil {
			if pmVal, ok := e.serviceToWorkload.Load(svcAndNS); ok {
				// For EndpointSlice watchers, the value is k8sclient.PodMetadata
				// For listPod approach, the value might be "workload@namespace"
				switch val := pmVal.(type) {
				case k8sclient.PodMetadata:
					return val.Workload, val.Namespace, nil
				case string:
					workload, namespace := k8sclient.ExtractResourceAndNamespace(val)
					return workload, namespace, nil
				default:
					e.logger.Debug("Unknown type in serviceToWorkload map")
				}
			}
		}
	}
	return "", "", errors.New("no kubernetes workload found for ip: " + ip)
}

func (e *kubernetesResolver) Process(attributes, resourceAttributes pcommon.Map) error {
	var namespace string
	if value, ok := attributes.Get(attr.AWSRemoteService); ok {
		valueStr := value.AsString()
		ipStr := ""
		if ip, _, ok := k8sclient.ExtractIPPort(valueStr); ok {
			if workload, ns, err := e.getWorkloadAndNamespaceByIP(valueStr); err == nil {
				attributes.PutStr(attr.AWSRemoteService, workload)
				namespace = ns
			} else {
				ipStr = ip
			}
		} else if k8sclient.IsIP(valueStr) {
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
