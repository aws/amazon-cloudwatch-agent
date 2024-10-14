// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	logger                         *zap.Logger
	clientset                      kubernetes.Interface
	clusterName                    string
	platformCode                   string
	ipToPod                        *sync.Map
	podToWorkloadAndNamespace      *sync.Map
	ipToServiceAndNamespace        *sync.Map
	serviceAndNamespaceToSelectors *sync.Map
	workloadAndNamespaceToLabels   *sync.Map
	serviceToWorkload              *sync.Map // computed from serviceAndNamespaceToSelectors and workloadAndNamespaceToLabels every 1 min
	workloadPodCount               map[string]int
	safeStopCh                     *safeChannel // trace and metric processors share the same kubernetesResolver and might close the same channel separately
}

// a safe channel which can be closed multiple times
type safeChannel struct {
	sync.Mutex

	ch     chan struct{}
	closed bool
}

func (sc *safeChannel) Close() {
	sc.Lock()
	defer sc.Unlock()

	if !sc.closed {
		close(sc.ch)
		sc.closed = true
	}
}

var (
	once     sync.Once
	instance *kubernetesResolver
)

func jitterSleep(seconds int) {
	jitter := time.Duration(rand.Intn(seconds)) * time.Second // nolint:gosec
	time.Sleep(jitter)
}

// Deleter represents a type that can delete a key from a map after a certain delay.
type Deleter interface {
	DeleteWithDelay(m *sync.Map, key interface{})
}

// TimedDeleter deletes a key after a specified delay.
type TimedDeleter struct {
	Delay time.Duration
}

func (td *TimedDeleter) DeleteWithDelay(m *sync.Map, key interface{}) {
	go func() {
		time.Sleep(td.Delay)
		m.Delete(key)
	}()
}

func (s *serviceWatcher) onAddOrUpdateService(service *corev1.Service) {
	// service can also have an external IP (or ingress IP) that could be accessed
	// this field can be either an IP address (in some edge case) or a hostname (see "EXTERNAL-IP" column in "k get svc" output)
	// [ec2-user@ip-172-31-11-104 one-step]$ k get svc -A
	// NAMESPACE           NAME                          TYPE           CLUSTER-IP       EXTERNAL-IP                                                              PORT(S)                                     AGE
	// default             pet-clinic-frontend           ClusterIP      10.100.216.182   <none>                                                                   8080/TCP                                    108m
	// default             vets-service                  ClusterIP      10.100.62.167    <none>                                                                   8083/TCP                                    108m
	// default             visits-service                ClusterIP      10.100.96.5      <none>                                                                   8082/TCP                                    108m
	// ingress-nginx       default-http-backend          ClusterIP      10.100.11.231    <none>                                                                   80/TCP                                      108m
	// ingress-nginx       ingress-nginx                 LoadBalancer   10.100.154.5     aex7997ece08c435dbd2b912fd5aa5bd-5372117830.xxxxx.elb.amazonaws.com      80:32080/TCP,443:32081/TCP,9113:30410/TCP   108m
	// kube-system         kube-dns                      ClusterIP      10.100.0.10      <none>
	//
	// we ignore such case for now and may need to consider it in the future
	if service.Spec.ClusterIP != "" && service.Spec.ClusterIP != corev1.ClusterIPNone {
		s.ipToServiceAndNamespace.Store(service.Spec.ClusterIP, getServiceAndNamespace(service))
	}
	labelSet := mapset.NewSet[string]()
	for key, value := range service.Spec.Selector {
		labelSet.Add(key + "=" + value)
	}
	if labelSet.Cardinality() > 0 {
		s.serviceAndNamespaceToSelectors.Store(getServiceAndNamespace(service), labelSet)
	}
}

func (s *serviceWatcher) onDeleteService(service *corev1.Service, deleter Deleter) {
	if service.Spec.ClusterIP != "" && service.Spec.ClusterIP != corev1.ClusterIPNone {
		deleter.DeleteWithDelay(s.ipToServiceAndNamespace, service.Spec.ClusterIP)
	}
	deleter.DeleteWithDelay(s.serviceAndNamespaceToSelectors, getServiceAndNamespace(service))
}

func (p *podWatcher) removeHostNetworkRecords(pod *corev1.Pod) {
	for _, port := range getHostNetworkPorts(pod) {
		p.deleter.DeleteWithDelay(p.ipToPod, pod.Status.HostIP+":"+port)
	}
}

func (p *podWatcher) handlePodAdd(pod *corev1.Pod) {
	if pod.Spec.HostNetwork && pod.Status.HostIP != "" {
		for _, port := range getHostNetworkPorts(pod) {
			p.ipToPod.Store(pod.Status.HostIP+":"+port, pod.Name)
		}
	}
	if pod.Status.PodIP != "" {
		p.ipToPod.Store(pod.Status.PodIP, pod.Name)
	}
}

func (p *podWatcher) handlePodUpdate(newPod *corev1.Pod, oldPod *corev1.Pod) {
	// HostNetwork is an immutable field
	if newPod.Spec.HostNetwork && oldPod.Status.HostIP != newPod.Status.HostIP {
		if oldPod.Status.HostIP != "" {
			p.logger.Debug("deleting host ip from cache", zap.String("hostNetwork", oldPod.Status.HostIP))
			p.removeHostNetworkRecords(oldPod)
		}
		if newPod.Status.HostIP != "" {
			for _, port := range getHostNetworkPorts(newPod) {
				p.ipToPod.Store(newPod.Status.HostIP+":"+port, newPod.Name)
			}
		}
	}
	if oldPod.Status.PodIP != newPod.Status.PodIP {
		if oldPod.Status.PodIP != "" {
			p.logger.Debug("deleting pod ip from cache", zap.String("podNetwork", oldPod.Status.PodIP))
			p.deleter.DeleteWithDelay(p.ipToPod, oldPod.Status.PodIP)
		}
		if newPod.Status.PodIP != "" {
			p.ipToPod.Store(newPod.Status.PodIP, newPod.Name)
		}
	}
}

func (p *podWatcher) onAddOrUpdatePod(pod, oldPod *corev1.Pod) {
	if oldPod == nil {
		p.handlePodAdd(pod)
	} else {
		p.handlePodUpdate(pod, oldPod)
	}

	workloadAndNamespace := getWorkloadAndNamespace(pod)

	if workloadAndNamespace != "" {
		p.podToWorkloadAndNamespace.Store(pod.Name, workloadAndNamespace)
		podLabels := mapset.NewSet[string]()
		for key, value := range pod.ObjectMeta.Labels {
			podLabels.Add(key + "=" + value)
		}
		if podLabels.Cardinality() > 0 {
			p.workloadAndNamespaceToLabels.Store(workloadAndNamespace, podLabels)
		}
		if oldPod == nil {
			p.workloadPodCount[workloadAndNamespace]++
			p.logger.Debug("Added pod", zap.String("pod", pod.Name), zap.String("workload", workloadAndNamespace), zap.Int("count", p.workloadPodCount[workloadAndNamespace]))
		}
	}
}

func (p *podWatcher) onDeletePod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	if pod.Spec.HostNetwork && pod.Status.HostIP != "" {
		p.logger.Debug("deleting host ip from cache", zap.String("hostNetwork", pod.Status.HostIP))
		p.removeHostNetworkRecords(pod)
	}
	if pod.Status.PodIP != "" {
		p.logger.Debug("deleting pod ip from cache", zap.String("podNetwork", pod.Status.PodIP))
		p.deleter.DeleteWithDelay(p.ipToPod, pod.Status.PodIP)
	}

	if workloadKey, ok := p.podToWorkloadAndNamespace.Load(pod.Name); ok {
		workloadAndNamespace := workloadKey.(string)
		p.workloadPodCount[workloadAndNamespace]--
		p.logger.Debug("decrementing pod count", zap.String("workload", workloadAndNamespace), zap.Int("podCount", p.workloadPodCount[workloadAndNamespace]))
		if p.workloadPodCount[workloadAndNamespace] == 0 {
			p.deleter.DeleteWithDelay(p.workloadAndNamespaceToLabels, workloadAndNamespace)
		}
	} else {
		p.logger.Error("failed to load pod workloadKey", zap.String("pod", pod.Name))
	}
	p.deleter.DeleteWithDelay(p.podToWorkloadAndNamespace, pod.Name)
}

type podWatcher struct {
	ipToPod                      *sync.Map
	podToWorkloadAndNamespace    *sync.Map
	workloadAndNamespaceToLabels *sync.Map
	workloadPodCount             map[string]int
	logger                       *zap.Logger
	informer                     cache.SharedIndexInformer
	deleter                      Deleter
}

func newPodWatcher(logger *zap.Logger, informer cache.SharedIndexInformer, deleter Deleter) *podWatcher {
	return &podWatcher{
		ipToPod:                      &sync.Map{},
		podToWorkloadAndNamespace:    &sync.Map{},
		workloadAndNamespaceToLabels: &sync.Map{},
		workloadPodCount:             make(map[string]int),
		logger:                       logger,
		informer:                     informer,
		deleter:                      deleter,
	}
}

func (p *podWatcher) run(stopCh chan struct{}) {
	p.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			p.logger.Debug("list and watch for pod: ADD " + pod.Name)
			p.onAddOrUpdatePod(pod, nil)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*corev1.Pod)
			oldPod := oldObj.(*corev1.Pod)
			p.logger.Debug("list and watch for pods: UPDATE " + pod.Name)
			p.onAddOrUpdatePod(pod, oldPod)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			p.logger.Debug("list and watch for pods: DELETE " + pod.Name)
			p.onDeletePod(obj)
		},
	})

	go p.informer.Run(stopCh)

}

func (p *podWatcher) waitForCacheSync(stopCh chan struct{}) {
	if !cache.WaitForNamedCacheSync("podWatcher", stopCh, p.informer.HasSynced) {
		p.logger.Fatal("timed out waiting for kubernetes pod watcher caches to sync")
	}

	p.logger.Info("podWatcher: Cache synced")
}

type serviceWatcher struct {
	ipToServiceAndNamespace        *sync.Map
	serviceAndNamespaceToSelectors *sync.Map
	logger                         *zap.Logger
	informer                       cache.SharedIndexInformer
	deleter                        Deleter
}

func newServiceWatcher(logger *zap.Logger, informer cache.SharedIndexInformer, deleter Deleter) *serviceWatcher {
	return &serviceWatcher{
		ipToServiceAndNamespace:        &sync.Map{},
		serviceAndNamespaceToSelectors: &sync.Map{},
		logger:                         logger,
		informer:                       informer,
		deleter:                        deleter,
	}
}

func (s *serviceWatcher) Run(stopCh chan struct{}) {
	s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			s.logger.Debug("list and watch for services: ADD " + service.Name)
			s.onAddOrUpdateService(service)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			service := newObj.(*corev1.Service)
			s.logger.Debug("list and watch for services: UPDATE " + service.Name)
			s.onAddOrUpdateService(service)
		},
		DeleteFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			s.logger.Debug("list and watch for services: DELETE " + service.Name)
			s.onDeleteService(service, s.deleter)
		},
	})
	go s.informer.Run(stopCh)
}

func (s *serviceWatcher) waitForCacheSync(stopCh chan struct{}) {
	if !cache.WaitForNamedCacheSync("serviceWatcher", stopCh, s.informer.HasSynced) {
		s.logger.Fatal("timed out waiting for kubernetes service watcher caches to sync")
	}

	s.logger.Info("serviceWatcher: Cache synced")
}

type serviceToWorkloadMapper struct {
	serviceAndNamespaceToSelectors *sync.Map
	workloadAndNamespaceToLabels   *sync.Map
	serviceToWorkload              *sync.Map
	logger                         *zap.Logger
	deleter                        Deleter
}

func newServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload *sync.Map, logger *zap.Logger, deleter Deleter) *serviceToWorkloadMapper {
	return &serviceToWorkloadMapper{
		serviceAndNamespaceToSelectors: serviceAndNamespaceToSelectors,
		workloadAndNamespaceToLabels:   workloadAndNamespaceToLabels,
		serviceToWorkload:              serviceToWorkload,
		logger:                         logger,
		deleter:                        deleter,
	}
}

func (m *serviceToWorkloadMapper) mapServiceToWorkload() {
	m.logger.Debug("Map service to workload at:", zap.Time("time", time.Now()))

	m.serviceAndNamespaceToSelectors.Range(func(key, value interface{}) bool {
		var workloads []string
		serviceAndNamespace := key.(string)
		_, serviceNamespace := extractResourceAndNamespace(serviceAndNamespace)
		serviceLabels := value.(mapset.Set[string])

		m.workloadAndNamespaceToLabels.Range(func(workloadKey, labelsValue interface{}) bool {
			labels := labelsValue.(mapset.Set[string])
			workloadAndNamespace := workloadKey.(string)
			_, workloadNamespace := extractResourceAndNamespace(workloadAndNamespace)
			if workloadNamespace == serviceNamespace && workloadNamespace != "" && serviceLabels.IsSubset(labels) {
				m.logger.Debug("Found workload for service", zap.String("service", serviceAndNamespace), zap.String("workload", workloadAndNamespace))
				workloads = append(workloads, workloadAndNamespace)
			}

			return true
		})

		if len(workloads) > 1 {
			m.logger.Info("Multiple workloads found for service. You will get unexpected results.", zap.String("service", serviceAndNamespace), zap.Strings("workloads", workloads))
		} else if len(workloads) == 1 {
			m.serviceToWorkload.Store(serviceAndNamespace, workloads[0])
		} else {
			m.logger.Debug("No workload found for service", zap.String("service", serviceAndNamespace))
			m.deleter.DeleteWithDelay(m.serviceToWorkload, serviceAndNamespace)
		}
		return true
	})
}

func (m *serviceToWorkloadMapper) Start(stopCh chan struct{}) {
	// do the first mapping immediately
	m.mapServiceToWorkload()
	m.logger.Debug("First-time map service to workload at:", zap.Time("time", time.Now()))

	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-time.After(time.Minute + 30*time.Second):
				m.mapServiceToWorkload()
				m.logger.Debug("Map service to workload at:", zap.Time("time", time.Now()))
			}
		}
	}()
}

// minimizePod removes fields that could contain large objects, and retain essential
// fields needed for IP/name translation. The following fields must be kept:
// - ObjectMeta: Namespace, Name, Labels, OwnerReference
// - Spec: HostNetwork, ContainerPorts
// - Status: PodIP/s, HostIP/s
func minimizePod(obj interface{}) (interface{}, error) {
	if pod, ok := obj.(*corev1.Pod); ok {
		pod.Annotations = nil
		pod.Finalizers = nil
		pod.ManagedFields = nil

		pod.Spec.Volumes = nil
		pod.Spec.InitContainers = nil
		pod.Spec.EphemeralContainers = nil
		pod.Spec.ImagePullSecrets = nil
		pod.Spec.HostAliases = nil
		pod.Spec.SchedulingGates = nil
		pod.Spec.ResourceClaims = nil
		pod.Spec.Tolerations = nil
		pod.Spec.Affinity = nil

		pod.Status.InitContainerStatuses = nil
		pod.Status.ContainerStatuses = nil
		pod.Status.EphemeralContainerStatuses = nil

		for i := 0; i < len(pod.Spec.Containers); i++ {
			c := &pod.Spec.Containers[i]
			c.Image = ""
			c.Command = nil
			c.Args = nil
			c.EnvFrom = nil
			c.Env = nil
			c.Resources = corev1.ResourceRequirements{}
			c.VolumeMounts = nil
			c.VolumeDevices = nil
			c.SecurityContext = nil
		}
	}
	return obj, nil
}

// minimizeService removes fields that could contain large objects, and retain essential
// fields needed for IP/name translation. The following fields must be kept:
// - ObjectMeta: Namespace, Name
// - Spec: Selectors, ClusterIP
func minimizeService(obj interface{}) (interface{}, error) {
	if svc, ok := obj.(*corev1.Service); ok {
		svc.Annotations = nil
		svc.Finalizers = nil
		svc.ManagedFields = nil

		svc.Spec.LoadBalancerSourceRanges = nil
		svc.Spec.SessionAffinityConfig = nil
		svc.Spec.IPFamilies = nil
		svc.Spec.IPFamilyPolicy = nil
		svc.Spec.InternalTrafficPolicy = nil
		svc.Spec.InternalTrafficPolicy = nil

		svc.Status.Conditions = nil
	}
	return obj, nil
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

		sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 0)
		podInformer := sharedInformerFactory.Core().V1().Pods().Informer()
		err = podInformer.SetTransform(minimizePod)
		if err != nil {
			logger.Error("failed to minimize Pod objects", zap.Error(err))
		}
		serviceInformer := sharedInformerFactory.Core().V1().Services().Informer()
		err = serviceInformer.SetTransform(minimizeService)
		if err != nil {
			logger.Error("failed to minimize Service objects", zap.Error(err))
		}

		timedDeleter := &TimedDeleter{Delay: deletionDelay}
		poWatcher := newPodWatcher(logger, podInformer, timedDeleter)
		svcWatcher := newServiceWatcher(logger, serviceInformer, timedDeleter)

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
			safeStopCh:                     safeStopCh,
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
	if podKey, ok := e.ipToPod.Load(ip); ok {
		pod := podKey.(string)
		if workloadKey, ok := e.podToWorkloadAndNamespace.Load(pod); ok {
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
