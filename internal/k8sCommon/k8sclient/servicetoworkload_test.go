// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"sync"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"go.uber.org/zap"
)

func TestMapServiceToWorkload(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	serviceAndNamespaceToSelectors := &sync.Map{}
	workloadAndNamespaceToLabels := &sync.Map{}
	serviceToWorkload := &sync.Map{}

	serviceAndNamespaceToSelectors.Store("service1@namespace1", mapset.NewSet("label1=value1", "label2=value2"))
	workloadAndNamespaceToLabels.Store("deployment1@namespace1", mapset.NewSet("label1=value1", "label2=value2", "label3=value3"))

	mapper := NewServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload, logger, mockDeleter)
	mapper.mapServiceToWorkload()

	if _, ok := serviceToWorkload.Load("service1@namespace1"); !ok {
		t.Errorf("Expected service1@namespace1 to be mapped to a workload, but it was not")
	}
}

func TestMapServiceToWorkload_NoWorkload(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	serviceAndNamespaceToSelectors := &sync.Map{}
	workloadAndNamespaceToLabels := &sync.Map{}
	serviceToWorkload := &sync.Map{}

	// Add a service with no matching workload
	serviceAndNamespace := "service@namespace"
	serviceAndNamespaceToSelectors.Store(serviceAndNamespace, mapset.NewSet("label1=value1"))
	serviceToWorkload.Store(serviceAndNamespace, "workload@namespace")

	mapper := NewServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload, logger, mockDeleter)
	mapper.mapServiceToWorkload()

	// Check that the service was deleted from serviceToWorkload
	if _, ok := serviceToWorkload.Load(serviceAndNamespace); ok {
		t.Errorf("Service was not deleted from serviceToWorkload")
	}
}

func TestMapServiceToWorkload_MultipleWorkloads(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	serviceAndNamespaceToSelectors := &sync.Map{}
	workloadAndNamespaceToLabels := &sync.Map{}
	serviceToWorkload := &sync.Map{}

	serviceAndNamespace := "service@namespace"
	serviceAndNamespaceToSelectors.Store(serviceAndNamespace, mapset.NewSet("label1=value1", "label2=value2"))

	// Add two workloads with matching labels to the service
	workloadAndNamespaceToLabels.Store("workload1@namespace", mapset.NewSet("label1=value1", "label2=value2", "label3=value3"))
	workloadAndNamespaceToLabels.Store("workload2@namespace", mapset.NewSet("label1=value1", "label2=value2", "label4=value4"))

	mapper := NewServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload, logger, mockDeleter)
	mapper.mapServiceToWorkload()

	// Check that the service does not map to any workload
	if _, ok := serviceToWorkload.Load(serviceAndNamespace); ok {
		t.Errorf("Unexpected mapping of service to multiple workloads")
	}
}

func TestStopsWhenSignaled(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	serviceAndNamespaceToSelectors := &sync.Map{}
	workloadAndNamespaceToLabels := &sync.Map{}
	serviceToWorkload := &sync.Map{}

	stopchan := make(chan struct{})

	// Signal the stopchan to stop after 100 milliseconds
	time.AfterFunc(100*time.Millisecond, func() {
		close(stopchan)
	})

	mapper := NewServiceToWorkloadMapper(serviceAndNamespaceToSelectors, workloadAndNamespaceToLabels, serviceToWorkload, logger, mockDeleter)

	start := time.Now()
	mapper.Start(stopchan)
	duration := time.Since(start)

	// Check that the function stopped in a reasonable time after the stop signal
	if duration > 200*time.Millisecond {
		t.Errorf("mapServiceToWorkload did not stop in a reasonable time after the stop signal, duration: %v", duration)
	}
}
