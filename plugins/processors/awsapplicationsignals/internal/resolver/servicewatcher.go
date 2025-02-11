// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

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
		UpdateFunc: func(_, newObj interface{}) {
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
