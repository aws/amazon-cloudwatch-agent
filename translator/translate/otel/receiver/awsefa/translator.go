// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsefa

import (
	"fmt"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultCollectionInterval = time.Minute
	efaPrefix                 = "efa_"
)

var (
	baseKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.EfaKey)
)

var _ common.ComponentTranslator = (*translator)(nil)

type translator struct {
	common.NameProvider
	factory receiver.Factory
}

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: awsefareceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*awsefareceiver.Config)

	intervalKeyChain := []string{
		common.ConfigKey(baseKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, defaultCollectionInterval)

	efaMap, ok := conf.Get(baseKey).(map[string]any)
	if !ok {
		return cfg, nil
	}

	if _, hasMeasurement := efaMap[common.MeasurementKey]; hasMeasurement {
		measurements := common.GetMeasurements(efaMap)
		metrics := getEnabledMeasurements(measurements)
		c := confmap.NewFromStringMap(map[string]any{
			"metrics": metrics,
		})
		if err := c.Unmarshal(&cfg); err != nil {
			return nil, fmt.Errorf("unable to unmarshal efa receiver (%s): %w", t.ID(), err)
		}
	}

	return cfg, nil
}

// allEfaMetrics is the complete list of EFA metrics for disabling unselected ones.
// Keep in sync with awsefareceiver/internal/metadata.MetricsConfig.
var allEfaMetrics = map[string]bool{
	"efa_impaired_remote_conn_events": true,
	"efa_rdma_read_bytes":             true,
	"efa_rdma_read_resp_bytes":        true,
	"efa_rdma_read_wr_err":            true,
	"efa_rdma_read_wrs":               true,
	"efa_rdma_write_bytes":            true,
	"efa_rdma_write_recv_bytes":       true,
	"efa_rdma_write_wr_err":           true,
	"efa_rdma_write_wrs":              true,
	"efa_recv_bytes":                  true,
	"efa_recv_wrs":                    true,
	"efa_retrans_bytes":               true,
	"efa_retrans_pkts":                true,
	"efa_retrans_timeout_events":      true,
	"efa_rx_bytes":                    true,
	"efa_rx_dropped":                  true,
	"efa_rx_pkts":                     true,
	"efa_send_bytes":                  true,
	"efa_send_wrs":                    true,
	"efa_tx_bytes":                    true,
	"efa_tx_pkts":                     true,
	"efa_unresponsive_remote_events":  true,
}

func getEnabledMeasurements(measurements []string) map[string]any {
	// Disable all metrics first.
	metrics := map[string]any{}
	for m := range allEfaMetrics {
		metrics[m] = map[string]any{"enabled": false}
	}
	// Enable only the selected ones that are valid EFA metrics.
	for _, m := range measurements {
		metricName := m
		if !strings.HasPrefix(m, efaPrefix) {
			metricName = efaPrefix + m
		}
		if allEfaMetrics[metricName] {
			metrics[metricName] = map[string]any{"enabled": true}
		}
	}
	return metrics
}
