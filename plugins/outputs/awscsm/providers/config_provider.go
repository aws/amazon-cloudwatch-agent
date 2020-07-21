// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

import (
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm"
	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm/csmiface"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/metametrics"
	"github.com/aws/aws-sdk-go/aws"
)

const (
	statusReady     = csm.ClientPublishingStatusTypeReady
	statusPaused    = csm.ClientPublishingStatusTypePaused
	statusSuspended = csm.ClientPublishingStatusTypeSuspended
	statusTerm      = csm.ClientPublishingStatusTypeTerminated

	compressionSizeLimit         = 5464
	uncompressedSamplesSizeLimit = 24576
)

var (
	// FallbackQueryInterval ...
	FallbackQueryInterval = int64(5)

	// DefaultInterval ...
	DefaultInterval = time.Duration(FallbackQueryInterval) * time.Minute
)

// ConfigProvider will allow the output plugin
// to retrieve specific configuration to change the
// behavior of the agent.
type ConfigProvider interface {
	RetrieveAgentConfig() AgentConfig
	Close()
}

// AgentConfig is the output configuration used to handle
// outgoing request
type AgentConfig struct {
	Endpoint      string
	SchemaVersion string
	Status        string
	Limits        Limits

	// schemas
	Definitions Definitions
}

func (c AgentConfig) String() string {
	return fmt.Sprintf("AgentConfig:\n\tEndpoint: %s\n\tSchemaVersion: %s\n\tStatus:%s\n\t%s\n\t%v",
		c.Endpoint,
		c.SchemaVersion,
		c.Status,
		c.Limits.String(),
		c.Definitions,
	)
}

// Limits ...
type Limits struct {
	MaxCompressedSampleSize         int
	MaxUncompressedSampleSize       int
	MaxSEHBuckets                   int
	MaxFrequencyDistributionKeySize int
	MaxAggregationKeyValueSize      int
	MaxFrequencyDistributionSize    int
	MaxRecords                      int
	MaxPublishingMetricsPerCall     int
}

func (l Limits) String() string {
	return fmt.Sprintf("Limits:\n\t\tMaxCompressedSampleSize: %d\n\t\tMaxUncompressedSampleSize: %d\n\t\tMaxSEHBuckets: %d\n\t\tMaxFrequencyDistributionKeySize: %d\n\t\tMaxAggregationKeyValueSize: %d\n\t\tMaxFrequencyDistributionSize: %d\n\t\tMaxRecords: %d\n\tMaxPublishingMetricsPerCall: %d",
		l.MaxCompressedSampleSize,
		l.MaxUncompressedSampleSize,
		l.MaxSEHBuckets,
		l.MaxFrequencyDistributionKeySize,
		l.MaxAggregationKeyValueSize,
		l.MaxFrequencyDistributionSize,
		l.MaxRecords,
		l.MaxPublishingMetricsPerCall,
	)
}

type agentConfigHandler struct {
	container atomic.Value
}

var defaultLimits = Limits{
	MaxCompressedSampleSize:         compressionSizeLimit,
	MaxUncompressedSampleSize:       uncompressedSamplesSizeLimit,
	MaxAggregationKeyValueSize:      96,
	MaxFrequencyDistributionSize:    60,
	MaxFrequencyDistributionKeySize: 255,
	MaxSEHBuckets:                   180,
	MaxRecords:                      5,
	MaxPublishingMetricsPerCall:     20,
}

func (handler *agentConfigHandler) Get() AgentConfig {
	v := handler.container.Load()
	switch cfg := v.(type) {
	case AgentConfig:
		return cfg
	}

	return AgentConfig{
		Status:      statusPaused,
		Limits:      defaultLimits,
		Definitions: DefaultDefinitions(),
	}
}

type csmConfigProvider struct {
	client             csmiface.CSMAPI
	agentConfigHandler *agentConfigHandler
	done               chan struct{}
}

// Config ...
var Config ConfigProvider

// NewCSMConfigProvider will return a config provider. Will also start an interval goroutine
// that will poll the config service and construct a new agent config from that.
func NewCSMConfigProvider(svc csmiface.CSMAPI, interval time.Duration) ConfigProvider {
	c := &csmConfigProvider{
		client:             svc,
		done:               make(chan struct{}),
		agentConfigHandler: &agentConfigHandler{},
	}

	go c.interval(interval)
	return c
}

func (c *csmConfigProvider) RetrieveAgentConfig() AgentConfig {
	return c.agentConfigHandler.Get()
}

func (c *csmConfigProvider) Close() {
	if c.done != nil {
		close(c.done)
		c.done = nil
	}
}

const (
	opPublishingConfigurationKey = "GetPublishingConfiguration"
	opPublishingSchemaKey        = "GetPublishingSchema"
)

// interval will run periodically, interval duration, to poll the service for latest
// agent configuration.
func (c *csmConfigProvider) interval(interval time.Duration) {
	// perform the very first publishing configuration query within at most a minute of Start
	startupInterval := interval
	if interval.Minutes() >= 1.0 {
		startupInterval = time.Duration(rand.Int63n(int64(60 * time.Second)))
	}

	t := time.NewTimer(startupInterval)

	cfg := AgentConfig{}
	queryIntervalInMinutes := int64(interval.Minutes())

	for {
		updated := false

		select {
		case <-c.done:
			return
		case <-t.C:
			log.Println("D! Output awscsm config provider ticking")
			cfg = c.agentConfigHandler.Get()
			out, err := c.client.GetPublishingConfiguration(nil)
			endpoint := ""
			apiCallTimestamp := time.Now()

			if svc, ok := c.client.(*csm.CSM); ok {
				endpoint = svc.Client.Endpoint
			}

			metametrics.MetricListener.CountSuccess(opPublishingConfigurationKey, err == nil, apiCallTimestamp, endpoint)

			if err != nil {
				log.Println("E!", err)
				break
			}

			if err := validateConfiguration(out); err != nil {
				log.Println("E!", err)
				break
			}

			queryIntervalInMinutes = *(out.QueryIntervalInMinutes)
			cfg.Endpoint = aws.StringValue(out.Endpoint)
			cfg.Status = aws.StringValue(out.Status)
			updated = true

			if cfg.SchemaVersion == *out.SchemaVersion {
				break
			}

			if cfg.IsTerminated() {
				break
			}

			cfg.SchemaVersion = aws.StringValue(out.SchemaVersion)
			schema, err := c.client.GetPublishingSchema(&csm.GetPublishingSchemaInput{
				SchemaVersion: out.SchemaVersion,
			})

			metametrics.MetricListener.CountSuccess(opPublishingSchemaKey, err == nil, apiCallTimestamp, endpoint)

			if err != nil {
				log.Println("E!", err)
				break
			}

			if err := validateSchema(schema); err != nil {
				log.Println("E!", err)
				break
			}

			cfg.Definitions.clear()
			cfg.Definitions.add(schema)

			cfg.Limits = Limits{
				MaxCompressedSampleSize:         int(aws.Int64Value(schema.ServiceLimits.CompressedEventSamplesSizeLimit)),
				MaxUncompressedSampleSize:       int(aws.Int64Value(schema.ServiceLimits.UncompressedSamplesLengthLimit)),
				MaxSEHBuckets:                   int(aws.Int64Value(schema.ServiceLimits.SehBucketLimit)),
				MaxFrequencyDistributionKeySize: int(aws.Int64Value(schema.ServiceLimits.FrequencyDistributionEntryKeySizeLimit)),
				MaxAggregationKeyValueSize:      int(aws.Int64Value(schema.ServiceLimits.SdkAggregationKeyEntryValueSizeLimit)),
				MaxFrequencyDistributionSize:    int(aws.Int64Value(schema.ServiceLimits.FrequencyMetricDistributionSizeLimit)),
				MaxRecords:                      int(aws.Int64Value(schema.ServiceLimits.SdkMonitoringRecordsLimit)),
				MaxPublishingMetricsPerCall:     int(aws.Int64Value(schema.ServiceLimits.PublishingMetricsLimit)),
			}
			log.Println("Config updated to", cfg)
		}

		if updated {
			c.agentConfigHandler.container.Store(cfg)
		}

		if cfg.IsTerminated() {
			continue
		}

		if queryIntervalInMinutes <= 1 {
			queryIntervalInMinutes = FallbackQueryInterval
		}

		timer := int64(time.Duration(queryIntervalInMinutes) * time.Minute)
		timerHalf := timer / 2
		interval = time.Duration(rand.Int63n(2 * timerHalf))
		interval += time.Duration(timerHalf)

		t.Reset(interval)
	}
}

var (
	errInvalidConfigurationEndpoint      = fmt.Errorf("E! invalid configuration endpoint")
	errInvalidConfigurationInterval      = fmt.Errorf("E! invalid configuration interval")
	errInvalidConfigurationSchemaVersion = fmt.Errorf("E! invalid configuration version")
	errInvalidConfigurationStatus        = fmt.Errorf("E! invalid configuration status")
)

// validateConfiguration ensures that necessary values aren't nil. If a value is nil,
// this method will return the appropriate error.
func validateConfiguration(out *csm.GetPublishingConfigurationOutput) error {
	if out.Endpoint == nil {
		return errInvalidConfigurationEndpoint
	}

	if out.QueryIntervalInMinutes == nil {
		return errInvalidConfigurationInterval
	}

	if out.SchemaVersion == nil {
		return errInvalidConfigurationSchemaVersion
	}

	if out.Status == nil {
		return errInvalidConfigurationStatus
	}

	return nil
}

var (
	errInvalidServiceLimits               = fmt.Errorf("E! invalid service limit")
	errInvalidCompressedEventSamplesSize  = fmt.Errorf("E! invalid compressed event samples size")
	errInvalidFreqDistKeySize             = fmt.Errorf("E! invalid frequency distribution key size")
	errInvalidFreqDistSize                = fmt.Errorf("E! invalid frequency distribution size")
	errInvalidAggregationKeySize          = fmt.Errorf("E! invalid aggregation key size")
	errInvalidRecordsLimit                = fmt.Errorf("E! invalid record limit")
	errInvalidSEHBucketLimit              = fmt.Errorf("E! invalid seh bucket limit")
	errInvalidUncompressedSamplesLimit    = fmt.Errorf("E! invalid uncompressed samples limit")
	errInvalidMaxPublishingMetricsPerCall = fmt.Errorf("E! invalid maximum publishing metrics per call limit")
)

// validateSchema ensures that necessary values aren't nil. If a value is nil,
// this method will return the appropriate error.
func validateSchema(out *csm.GetPublishingSchemaOutput) error {
	if out.ServiceLimits == nil {
		return errInvalidServiceLimits
	}

	if out.ServiceLimits.CompressedEventSamplesSizeLimit == nil ||
		*out.ServiceLimits.CompressedEventSamplesSizeLimit < 0 {
		return errInvalidCompressedEventSamplesSize
	}

	if out.ServiceLimits.FrequencyDistributionEntryKeySizeLimit == nil ||
		*out.ServiceLimits.FrequencyDistributionEntryKeySizeLimit < 0 {
		return errInvalidFreqDistKeySize
	}

	if out.ServiceLimits.FrequencyMetricDistributionSizeLimit == nil ||
		*out.ServiceLimits.FrequencyMetricDistributionSizeLimit < 0 {
		return errInvalidFreqDistSize
	}

	if out.ServiceLimits.SdkAggregationKeyEntryValueSizeLimit == nil ||
		*out.ServiceLimits.SdkAggregationKeyEntryValueSizeLimit < 0 {
		return errInvalidAggregationKeySize
	}

	if out.ServiceLimits.SdkMonitoringRecordsLimit == nil ||
		*out.ServiceLimits.SdkMonitoringRecordsLimit <= 0 {
		return errInvalidRecordsLimit
	}

	if out.ServiceLimits.SehBucketLimit == nil ||
		*out.ServiceLimits.SehBucketLimit < 0 {
		return errInvalidSEHBucketLimit
	}

	if out.ServiceLimits.UncompressedSamplesLengthLimit == nil ||
		*out.ServiceLimits.UncompressedSamplesLengthLimit < 0 {
		return errInvalidUncompressedSamplesLimit
	}

	if out.ServiceLimits.PublishingMetricsLimit == nil ||
		*out.ServiceLimits.PublishingMetricsLimit <= 0 {
		return errInvalidMaxPublishingMetricsPerCall
	}

	return nil
}

// CanCollect will return whether or not collecting of metrics should occur.
func (c AgentConfig) CanCollect() bool {
	return c.Status == statusPaused ||
		c.Status == statusReady
}

// CanPublish will return whether or not publishing of metrics should occur.
func (c AgentConfig) CanPublish() bool {
	return c.Status == statusReady
}

func (c AgentConfig) IsTerminated() bool {
	return c.Status == statusTerm
}

// While the logic is currently equivalent to CanCollect's, this is a separate concept
func (c AgentConfig) ShouldPublishInternalMetrics() bool {
	return c.Status == statusPaused || c.Status == statusReady
}
