// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cardinalitycontrol

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
)

const (
	UnprocessedServiceOperationValue       = "AllOtherOperations"
	UnprocessedRemoteServiceOperationValue = "AllOtherRemoteOperations"
)

const (
	defaultCMSDepth = 3
	defaultCMSWidth = 5000
)

var awsDeclaredMetricAttributes = []string{
	common.AttributeEKSClusterName,
	common.AttributeK8SNamespace,
	common.MetricAttributeEnvironment,
	common.MetricAttributeLocalService,
	common.MetricAttributeLocalOperation,
	common.MetricAttributeRemoteService,
	common.MetricAttributeRemoteOperation,
	common.MetricAttributeRemoteResourceIdentifier,
	common.MetricAttributeRemoteEnvironment,
	common.AttributeK8SClusterName,
}

type Limiter interface {
	Admit(name string, attributes, resourceAttributes pcommon.Map) (bool, error)
}

type MetricsLimiter struct {
	DropThreshold     int
	LogDroppedMetrics bool
	RotationInterval  time.Duration

	logger   *zap.Logger
	ctx      context.Context
	mapLock  sync.RWMutex
	services map[string]*service
}

func NewMetricsLimiter(config *config.LimiterConfig, logger *zap.Logger) Limiter {
	logger.Info("creating metrics limiter with config", zap.Any("config", config))

	ctx := config.ParentContext
	if ctx == nil {
		ctx = context.TODO()
	}

	limiter := &MetricsLimiter{
		DropThreshold:     config.Threshold,
		LogDroppedMetrics: config.LogDroppedMetrics,
		RotationInterval:  config.RotationInterval,

		logger:   logger,
		ctx:      ctx,
		services: map[string]*service{},
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				limiter.removeStaleServices()
				time.Sleep(config.GarbageCollectionInterval)
			}
		}
	}()

	logger.Info("metrics limiter created.")

	return limiter
}

func (m *MetricsLimiter) Admit(metricName string, attributes, resourceAttributes pcommon.Map) (bool, error) {
	labels, serviceName, found := m.filterAWSDeclaredAttributes(attributes, resourceAttributes)
	if !found {
		return true, nil
	}
	admitted := true

	m.mapLock.RLock()
	svc := m.services[serviceName]
	m.mapLock.RUnlock()
	if svc == nil {
		m.mapLock.Lock()
		svc = m.services[serviceName]
		if svc == nil {
			svc = newService(serviceName, m.DropThreshold, m.RotationInterval, m.ctx, m.logger)
			m.services[serviceName] = svc
		}
		m.mapLock.Unlock()
	}

	metricData := newMetricData(serviceName, metricName, labels)

	reserved, _ := attributes.Get(common.AttributeTmpReserved)
	if reserved.Bool() {
		attributes.Remove(common.AttributeTmpReserved)
		return true, nil
	}

	if !svc.admitMetricData(metricData) {
		svc.rollupMetricData(attributes)

		svc.totalRollup++
		admitted = false

		if m.LogDroppedMetrics {
			m.logger.Debug(fmt.Sprintf("[%s] drop metric data", svc.name), zap.Any("labels", labels))
		}
	}

	svc.totalMetricSent++

	svc.rwLock.RLock()
	defer svc.rwLock.RUnlock()

	svc.totalCount++
	svc.InsertMetricDataToPrimary(metricData)
	svc.InsertMetricDataToSecondary(metricData)
	return admitted, nil
}

func (m *MetricsLimiter) filterAWSDeclaredAttributes(attributes, resourceAttributes pcommon.Map) (map[string]string, string, bool) {
	svcNameAttr, exists := attributes.Get(common.MetricAttributeLocalService)
	if !exists {
		return nil, "", false
	}
	labels := map[string]string{}
	svcName := svcNameAttr.AsString()
	for _, attrKey := range awsDeclaredMetricAttributes {
		if attr, ok := attributes.Get(attrKey); ok {
			labels[attrKey] = attr.AsString()
		}
	}
	return labels, svcName, true
}

func (m *MetricsLimiter) removeStaleServices() {
	var svcToRemove []string
	for name, svc := range m.services {
		if svc.rotations > 3 {
			if svc.countSnapshot[0] == svc.countSnapshot[1] && svc.countSnapshot[1] == svc.countSnapshot[2] {
				svc.cancelFunc()
				svcToRemove = append(svcToRemove, name)
			}
		}
	}

	m.mapLock.Lock()
	defer m.mapLock.Unlock()

	for _, name := range svcToRemove {
		m.logger.Info("remove stale service " + name + ".")
		delete(m.services, name)
	}
}

type service struct {
	logger     *zap.Logger
	name       string
	cancelFunc context.CancelFunc

	rwLock        sync.RWMutex
	primaryCMS    *CountMinSketch
	primaryTopK   *topKMetrics
	secondaryCMS  *CountMinSketch
	secondaryTopK *topKMetrics

	totalCount    int
	rotations     int
	countSnapshot []int

	totalRollup     int
	totalMetricSent int
}

func (s *service) InsertMetricDataToPrimary(md *MetricData) {
	s.primaryCMS.Insert(md)
	updatedFrequency := s.primaryCMS.Get(md)
	updatedMd := copyMetricDataWithUpdatedFrequency(md, updatedFrequency)
	s.primaryTopK.Push(md, updatedMd)
}

func (s *service) InsertMetricDataToSecondary(md *MetricData) {
	if s.secondaryCMS != nil {
		s.secondaryCMS.Insert(md)
		updatedFrequency := s.secondaryCMS.Get(md)
		updatedMd := copyMetricDataWithUpdatedFrequency(md, updatedFrequency)
		s.secondaryTopK.Push(md, updatedMd)
	}
}

// MetricData represents a key-value pair.
type MetricData struct {
	hashKey   string
	name      string
	service   string
	frequency int
}

func (m MetricData) HashKey() string {
	return m.hashKey
}

func (m MetricData) Frequency() int {
	return m.frequency
}

func newMetricData(serviceName, metricName string, labels map[string]string) *MetricData {
	hashID := sortAndConcatLabels(labels)
	return &MetricData{
		hashKey:   hashID,
		name:      metricName,
		service:   serviceName,
		frequency: 1,
	}
}

func copyMetricDataWithUpdatedFrequency(md *MetricData, frequency int) *MetricData {
	return &MetricData{
		hashKey:   md.hashKey,
		name:      md.name,
		service:   md.service,
		frequency: frequency,
	}
}

func sortAndConcatLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var concatenatedLabels string
	for _, key := range keys {
		concatenatedLabels += labels[key]
	}
	keys = nil
	return concatenatedLabels
}

// topKMetrics represents the priority queue with a map for key lookup and a size limit.
type topKMetrics struct {
	metricMap map[string]*MetricData
	minMetric *MetricData
	sizeLimit int
}

// newTopKMetrics creates a new topKMetrics with a specified size limit.
func newTopKMetrics(sizeLimit int) *topKMetrics {
	return &topKMetrics{
		metricMap: make(map[string]*MetricData),
		minMetric: nil,
		sizeLimit: sizeLimit,
	}
}

// Push adds a key-value pair to the priority queue. If the value already exists, it updates the frequency.
func (t *topKMetrics) Push(oldMetric, newMetric *MetricData) {
	hashValue := oldMetric.hashKey
	if t.minMetric == nil {
		t.minMetric = oldMetric
	}

	_, found := t.metricMap[hashValue]
	if found {
		// Update the frequency.
		t.metricMap[hashValue].frequency = newMetric.frequency
		// Check if this oldMetric is the new minimum, find the new minMetric after the updates
		if t.minMetric.hashKey == hashValue {
			// Find the new minMetrics after update the frequency
			t.minMetric = t.findMinMetric()
		}
		return
	}

	// If exceeded size limit, delete the smallest
	if len(t.metricMap) >= t.sizeLimit {
		if newMetric.frequency > t.minMetric.frequency {
			delete(t.metricMap, t.minMetric.hashKey)
			t.metricMap[hashValue] = newMetric
			t.minMetric = t.findMinMetric()
		}
	} else {
		// Check if this newMetric is the new minimum.
		if newMetric.frequency < t.minMetric.frequency {
			t.minMetric = newMetric
		}
		t.metricMap[hashValue] = newMetric
	}
}

// findMinMetric removes and returns the key-value pair with the minimum value.
func (t *topKMetrics) findMinMetric() *MetricData {
	// Find the new minimum metric and smallest frequency.
	var newMinMetric *MetricData
	smallestFrequency := int(^uint(0) >> 1) // Initialize with the maximum possible integer value

	for _, metric := range t.metricMap {
		if metric.frequency < smallestFrequency {
			smallestFrequency = metric.frequency
			newMinMetric = metric
		}
	}
	return newMinMetric
}

func (s *service) admitMetricData(metric *MetricData) bool {
	_, found := s.primaryTopK.metricMap[metric.hashKey]
	if len(s.primaryTopK.metricMap) < s.primaryTopK.sizeLimit || found {
		return true
	}
	return false
}

func (s *service) rollupMetricData(attributes pcommon.Map) {
	for _, indexAttr := range awsDeclaredMetricAttributes {
		if (indexAttr == common.MetricAttributeEnvironment) || (indexAttr == common.MetricAttributeLocalService) || (indexAttr == common.MetricAttributeRemoteService) {
			continue
		}
		if indexAttr == common.MetricAttributeLocalOperation {
			attributes.PutStr(indexAttr, UnprocessedServiceOperationValue)
		} else if indexAttr == common.MetricAttributeRemoteOperation {
			attributes.PutStr(indexAttr, UnprocessedRemoteServiceOperationValue)
		} else {
			attributes.PutStr(indexAttr, "-")
		}
	}
}

// As a starting point, you can use rules of thumb, such as setting the depth to be around 4-6 times the logarithm of the expected number of distinct items and the width based on your memory constraints. However, these are rough guidelines, and the optimal size will depend on your unique application and requirements.
func newService(name string, limit int, rotationInterval time.Duration, parentCtx context.Context, logger *zap.Logger) *service {
	depth := defaultCMSDepth
	width := defaultCMSWidth

	ctx, cancel := context.WithCancel(parentCtx)
	svc := &service{
		logger:        logger,
		name:          name,
		cancelFunc:    cancel,
		primaryCMS:    NewCountMinSketch(depth, width),
		primaryTopK:   newTopKMetrics(limit),
		countSnapshot: make([]int, 3),
	}

	// Create a ticker to create a new countMinSketch every 1 hour
	rotationTicker := time.NewTicker(rotationInterval)
	//defer rotationTicker.Stop()

	// Create a goroutine to handle rotationTicker.C
	go func() {
		for {
			select {
			case <-rotationTicker.C:
				svc.logger.Info(fmt.Sprintf("[%s] rotating visit records, current rotation %d", name, svc.rotations))
				if err := rotateVisitRecords(svc); err != nil {
					svc.logger.Error(fmt.Sprintf("[%s] failed to rotate visit records.", name), zap.Error(err))
				}
			case <-ctx.Done():
				return
			default:
				// Continue running the main program
				time.Sleep(1 * time.Second)
			}
		}
	}()

	svc.logger.Info(fmt.Sprintf("[%s] service entry is created.\n", name))
	return svc
}

func rotateVisitRecords(svc *service) error {
	svc.rwLock.Lock()
	defer svc.rwLock.Unlock()

	cmsDepth := svc.primaryCMS.depth
	cmsWidth := svc.primaryCMS.width
	topKLimit := svc.primaryTopK.sizeLimit

	nextPrimaryCMS := svc.secondaryCMS
	nextPrimaryTopK := svc.secondaryTopK

	svc.secondaryCMS = NewCountMinSketch(cmsDepth, cmsWidth)
	svc.secondaryTopK = newTopKMetrics(topKLimit)

	if nextPrimaryCMS != nil && nextPrimaryTopK != nil {
		svc.primaryCMS = nextPrimaryCMS
		svc.primaryTopK = nextPrimaryTopK
	} else {
		svc.logger.Info(fmt.Sprintf("[%s] secondary visit records are nil.", svc.name))
	}

	svc.countSnapshot[svc.rotations%3] = svc.totalCount
	svc.rotations++

	return nil
}
