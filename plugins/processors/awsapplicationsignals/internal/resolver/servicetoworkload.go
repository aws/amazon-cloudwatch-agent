// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"go.uber.org/zap"
)

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
