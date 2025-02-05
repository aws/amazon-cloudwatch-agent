// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"sync"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newServiceWatcherForTesting(ipToServiceAndNamespace, serviceAndNamespaceToSelectors *sync.Map) *serviceWatcher {
	logger, _ := zap.NewDevelopment()
	return &serviceWatcher{ipToServiceAndNamespace, serviceAndNamespaceToSelectors, logger, nil, nil}
}

func TestOnAddOrUpdateService(t *testing.T) {
	// Create a fake service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myservice",
			Namespace: "mynamespace",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1.2.3.4",
			Selector: map[string]string{
				"app": "myapp",
			},
		},
	}

	// Create the maps
	ipToServiceAndNamespace := &sync.Map{}
	serviceAndNamespaceToSelectors := &sync.Map{}

	// Call the function
	svcWatcher := newServiceWatcherForTesting(ipToServiceAndNamespace, serviceAndNamespaceToSelectors)
	svcWatcher.onAddOrUpdateService(service)

	// Check that the maps contain the expected entries
	if _, ok := ipToServiceAndNamespace.Load("1.2.3.4"); !ok {
		t.Errorf("ipToServiceAndNamespace does not contain the service IP")
	}
	if _, ok := serviceAndNamespaceToSelectors.Load("myservice@mynamespace"); !ok {
		t.Errorf("serviceAndNamespaceToSelectors does not contain the service")
	}
}

func TestOnDeleteService(t *testing.T) {
	// Create a fake service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myservice",
			Namespace: "mynamespace",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1.2.3.4",
			Selector: map[string]string{
				"app": "myapp",
			},
		},
	}

	// Create the maps and add the service to them
	ipToServiceAndNamespace := &sync.Map{}
	ipToServiceAndNamespace.Store("1.2.3.4", "myservice@mynamespace")
	serviceAndNamespaceToSelectors := &sync.Map{}
	serviceAndNamespaceToSelectors.Store("myservice@mynamespace", mapset.NewSet("app=myapp"))

	// Call the function
	svcWatcher := newServiceWatcherForTesting(ipToServiceAndNamespace, serviceAndNamespaceToSelectors)
	svcWatcher.onDeleteService(service, mockDeleter)

	// Check that the maps do not contain the service
	if _, ok := ipToServiceAndNamespace.Load("1.2.3.4"); ok {
		t.Errorf("ipToServiceAndNamespace still contains the service IP")
	}
	if _, ok := serviceAndNamespaceToSelectors.Load("myservice@mynamespace"); ok {
		t.Errorf("serviceAndNamespaceToSelectors still contains the service")
	}
}

func TestFilterServiceIPFields(t *testing.T) {
	meta := metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
	}
	svc := &corev1.Service{
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"name": "app",
			},
			ClusterIP: "10.0.12.4",
		},
	}
	newSvc, err := minimizeService(svc)
	assert.Nil(t, err)
	assert.Equal(t, "10.0.12.4", newSvc.(*corev1.Service).Spec.ClusterIP)
	assert.Equal(t, "app", newSvc.(*corev1.Service).Spec.Selector["name"])
}
