// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpu

import (
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type logTypeAttribute struct {
	logger *zap.Logger
}

func NewLogTypeAttribute(logger *zap.Logger) *logTypeAttribute {
	return &logTypeAttribute{
		logger: logger,
	}
}

func (an *logTypeAttribute) Process(m pmetric.Metric, attributes pcommon.Map, removeOriginal bool) error {
	an.addLogTypeAttribute(m, attributes)
	return nil
}

// NOTE: There are additional metric types (PodGpu and NodeGpu) that get applied in the emf exporter.
// Those 2 metric types handled by emf exporter are used only for dimensions sets that include "GpuDevice"
func (an *logTypeAttribute) addLogTypeAttribute(m pmetric.Metric, attributes pcommon.Map) {
	logType := ""
	switch strings.Split(m.Name(), "_")[0] {
	case "container":
		logType = containerinsightscommon.TypeContainer
	case "pod":
		logType = containerinsightscommon.TypePod
	case "node":
		logType = containerinsightscommon.TypeNode
	case "cluster":
		logType = containerinsightscommon.TypeCluster
	default:
		an.logger.Warn("metric name is either empty or not a supported type")
	}
	attributes.PutStr("Type", logType)
}
