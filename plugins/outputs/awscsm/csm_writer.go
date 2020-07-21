// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"fmt"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm"
	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm/csmiface"
	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/metametrics"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/providers"
)

const (
	maxPendingMetrics           = 1000
	putPublishingMetricsApiCall = "PutPublishingMetrics"
)

var errNonPositiveMetricCountLimit = fmt.Errorf("Metric count limit must be positive")
var errInvalidClientConfiguration = fmt.Errorf("Csm control plane client with no endpoint")

// CSMWriter used for the metametrics listener
type CSMWriter struct {
	svc            csmiface.CSMAPI
	env            csm.HostEnvironment
	pendingMetrics []*csm.PublishingMetric
	apiCallLimit   int64
}

// NewCSMWriter return a new CSM writer
func NewCSMWriter(svc csmiface.CSMAPI, env csm.HostEnvironment) *CSMWriter {
	return &CSMWriter{
		svc:            svc,
		env:            env,
		pendingMetrics: []*csm.PublishingMetric{},
		apiCallLimit:   1,
	}
}

func (writer *CSMWriter) Write(metrics metametrics.Metrics) error {
	config := providers.Config.RetrieveAgentConfig()
	if config.IsTerminated() {
		return nil
	}

	vals := adaptToCSMMetrics(metrics)
	writer.pendingMetrics = append(writer.pendingMetrics, vals...)

	excessMetrics := len(writer.pendingMetrics) - maxPendingMetrics
	if excessMetrics > 0 {
		writer.pendingMetrics = writer.pendingMetrics[excessMetrics:]
	}

	if !config.ShouldPublishInternalMetrics() {
		return nil
	}

	metricCountLimit := config.Limits.MaxPublishingMetricsPerCall
	if metricCountLimit <= 0 {
		return errNonPositiveMetricCountLimit
	}

	op := putPublishingMetricsApiCall
	endpoint := ""

	if svc, ok := writer.svc.(*csm.CSM); ok {
		endpoint = svc.Client.Endpoint
	} else {
		return errInvalidClientConfiguration // this should never happen
	}

	var calls int64 = 0
	errorOccurred := false

	for len(writer.pendingMetrics) > 0 && calls < writer.apiCallLimit {
		apiCallTimestamp := time.Now()
		metricCount := len(writer.pendingMetrics)
		if metricCount > metricCountLimit {
			metricCount = metricCountLimit
		}

		_, err := writer.svc.PutPublishingMetrics(&csm.PutPublishingMetricsInput{
			HostEnvironment: &writer.env,
			Metrics:         writer.pendingMetrics[:metricCount],
		})

		metametrics.MetricListener.CountSuccess(op, err == nil, apiCallTimestamp, endpoint)

		if err != nil {
			errorOccurred = true
			break
		}

		writer.pendingMetrics = writer.pendingMetrics[metricCount:]
		calls++
	}

	writer.updateApiCallLimit(errorOccurred)

	return nil
}

// Additive-increase, multiplicative-decrease (AIMD) rate control based on submission errors
func (writer *CSMWriter) updateApiCallLimit(errorOccurred bool) {
	if errorOccurred {
		writer.apiCallLimit = writer.apiCallLimit / 2
		if writer.apiCallLimit < 1 {
			writer.apiCallLimit = 1
		}
	} else {
		writer.apiCallLimit++
	}
}

func adaptToCSMMetrics(metrics metametrics.Metrics) []*csm.PublishingMetric {
	vals := make([]*csm.PublishingMetric, len(metrics))

	i := 0
	for _, v := range metrics {
		metric := v
		vals[i] = &csm.PublishingMetric{
			Name:      &metric.Key.Name,
			Endpoint:  &metric.Key.Endpoint,
			Timestamp: aws.Time(metric.Key.Timestamp),
			StatisticSet: &csm.StatisticSet{
				SampleCount: &metric.Stats.SampleCount,
				Sum:         &metric.Stats.Sum,
				Minimum:     &metric.Stats.Min,
				Maximum:     &metric.Stats.Max,
			},
		}
		i++
	}

	return vals
}
