// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm/csm"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/awscsm/metametrics"
)

type mockCSMService struct {
	*csm.CSM
	GetPublishingConfigurationFn func() (*csm.GetPublishingConfigurationOutput, error)
	GetPublishingSchemaFn        func() (*csm.GetPublishingSchemaOutput, error)
}

func (c *mockCSMService) GetPublishingConfiguration(input *csm.GetPublishingConfigurationInput) (*csm.GetPublishingConfigurationOutput, error) {
	if c.GetPublishingConfigurationFn != nil {
		return c.GetPublishingConfigurationFn()
	}

	return &csm.GetPublishingConfigurationOutput{}, nil
}

func (c *mockCSMService) GetPublishingSchema(input *csm.GetPublishingSchemaInput) (*csm.GetPublishingSchemaOutput, error) {
	if c.GetPublishingSchemaFn != nil {
		return c.GetPublishingSchemaFn()
	}

	return &csm.GetPublishingSchemaOutput{}, nil
}

func TestConfigProviderDone(t *testing.T) {
	p := NewCSMConfigProvider(&mockCSMService{}, DefaultInterval)
	provider := p.(*csmConfigProvider)

	provider.Close()
	if provider.done != nil {
		t.Errorf("expected done channel to be nil, but wasn't")
	}

	// ensure multiple calls do not panic
	provider.Close()
	provider.Close()
	provider.Close()
}

func TestConfigProviderGettingConfiguration(t *testing.T) {
	cases := []struct {
		name                   string
		interval               time.Duration
		ConfigFn               func() (*csm.GetPublishingConfigurationOutput, error)
		SchemaFn               func() (*csm.GetPublishingSchemaOutput, error)
		expectedAgentConfig    AgentConfig
		differentSchemaVersion bool
	}{
		{
			name:     "valid case",
			interval: 50 * time.Millisecond,
			ConfigFn: func() (*csm.GetPublishingConfigurationOutput, error) {
				return &csm.GetPublishingConfigurationOutput{
					Endpoint:               aws.String("foo.bar"),
					Status:                 aws.String("baz"),
					SchemaVersion:          aws.String("1"),
					QueryIntervalInMinutes: aws.Int64(1),
				}, nil
			},
			SchemaFn: func() (*csm.GetPublishingSchemaOutput, error) {
				return &csm.GetPublishingSchemaOutput{
					MonitoringEventEntrySchemas: []*csm.MonitoringEventEntrySchema{
						{
							Name:    aws.String("foo_key_type"),
							KeyType: aws.String(csm.MonitoringEventEntryKeyTypeAggregation),
						},
						{
							Name:       aws.String("frequency_metric"),
							MetricType: aws.String(csm.MonitoringEventEntryMetricTypeFrequency),
						},
						{
							Name:       aws.String("seh_metric"),
							MetricType: aws.String(csm.MonitoringEventEntryMetricTypeSeh),
						},
					},
					ServiceLimits: &csm.ServiceLimits{
						CompressedEventSamplesSizeLimit:        aws.Int64(10000),
						FrequencyDistributionEntryKeySizeLimit: aws.Int64(255),
						FrequencyMetricDistributionSizeLimit:   aws.Int64(10000),
						SdkAggregationKeyEntryValueSizeLimit:   aws.Int64(10000),
						SdkMonitoringRecordsLimit:              aws.Int64(5),
						SehBucketLimit:                         aws.Int64(100),
						UncompressedSamplesLengthLimit:         aws.Int64(10000),
						PublishingMetricsLimit:                 aws.Int64(30),
					},
				}, nil
			},
			expectedAgentConfig: AgentConfig{
				Endpoint:      "foo.bar",
				Status:        "baz",
				SchemaVersion: "1",
				Definitions: Definitions{
					Entries: EventEntryDefinitions{
						container: map[string]EventEntryDefinition{
							"foo_key_type": {
								KeyType: EventEntryKeyType(csm.MonitoringEventEntryKeyTypeAggregation),
								Type:    MetricType(csm.MonitoringEventEntryMetricTypeNone),
								Name:    "foo_key_type",
							},
							"frequency_metric": {
								KeyType: EventEntryKeyType(csm.MonitoringEventEntryKeyTypeNone),
								Type:    MetricType(csm.MonitoringEventEntryMetricTypeFrequency),
								Name:    "frequency_metric",
							},
							"seh_metric": {
								KeyType: EventEntryKeyType(csm.MonitoringEventEntryKeyTypeNone),
								Type:    MetricType(csm.MonitoringEventEntryMetricTypeSeh),
								Name:    "seh_metric",
							},
						},
					},
					Events: EventDefinitions{
						container: map[string]EventDefinition{},
					},
				},
				Limits: Limits{
					MaxCompressedSampleSize:         10000,
					MaxUncompressedSampleSize:       10000,
					MaxSEHBuckets:                   100,
					MaxFrequencyDistributionKeySize: 255,
					MaxAggregationKeyValueSize:      10000,
					MaxFrequencyDistributionSize:    10000,
					MaxRecords:                      5,
					MaxPublishingMetricsPerCall:     30,
				},
			},
			differentSchemaVersion: true,
		},
		{
			name:     "invalid config",
			interval: 50 * time.Millisecond,
			ConfigFn: func() (*csm.GetPublishingConfigurationOutput, error) {
				return &csm.GetPublishingConfigurationOutput{
					QueryIntervalInMinutes: aws.Int64(1),
				}, nil
			},
			SchemaFn: func() (*csm.GetPublishingSchemaOutput, error) {
				return &csm.GetPublishingSchemaOutput{
					ServiceLimits: &csm.ServiceLimits{
						CompressedEventSamplesSizeLimit:        aws.Int64(1),
						FrequencyDistributionEntryKeySizeLimit: aws.Int64(2),
						FrequencyMetricDistributionSizeLimit:   aws.Int64(3),
						SdkAggregationKeyEntryValueSizeLimit:   aws.Int64(4),
						SdkMonitoringRecordsLimit:              aws.Int64(5),
						SehBucketLimit:                         aws.Int64(6),
						UncompressedSamplesLengthLimit:         aws.Int64(7),
						PublishingMetricsLimit:                 aws.Int64(8),
					},
				}, nil
			},
			expectedAgentConfig: AgentConfig{
				Status:      statusPaused,
				Limits:      defaultLimits,
				Definitions: DefaultDefinitions(),
			},
		},
		{
			name:                   "invalid schema",
			interval:               50 * time.Millisecond,
			differentSchemaVersion: true,
			ConfigFn: func() (*csm.GetPublishingConfigurationOutput, error) {
				return &csm.GetPublishingConfigurationOutput{
					Endpoint:               aws.String("foo.bar"),
					Status:                 aws.String("baz"),
					SchemaVersion:          aws.String("1"),
					QueryIntervalInMinutes: aws.Int64(1),
				}, nil
			},
			SchemaFn: func() (*csm.GetPublishingSchemaOutput, error) {
				return &csm.GetPublishingSchemaOutput{
					ServiceLimits: &csm.ServiceLimits{
						CompressedEventSamplesSizeLimit: aws.Int64(10000),
					},
				}, nil
			},
			expectedAgentConfig: AgentConfig{
				Status:        "baz",
				Endpoint:      "foo.bar",
				SchemaVersion: "1",
				Limits:        defaultLimits,
				Definitions:   DefaultDefinitions(),
			},
		},
		{
			name:     "terminal config",
			interval: 50 * time.Millisecond,
			ConfigFn: func() (*csm.GetPublishingConfigurationOutput, error) {
				return &csm.GetPublishingConfigurationOutput{
					Endpoint:               aws.String(""),
					Status:                 aws.String(statusTerm),
					SchemaVersion:          aws.String("1"),
					QueryIntervalInMinutes: aws.Int64(-1),
				}, nil
			},
			SchemaFn: func() (*csm.GetPublishingSchemaOutput, error) {
				return &csm.GetPublishingSchemaOutput{
					ServiceLimits: &csm.ServiceLimits{
						CompressedEventSamplesSizeLimit:        aws.Int64(1),
						FrequencyDistributionEntryKeySizeLimit: aws.Int64(2),
						FrequencyMetricDistributionSizeLimit:   aws.Int64(3),
						SdkAggregationKeyEntryValueSizeLimit:   aws.Int64(4),
						SdkMonitoringRecordsLimit:              aws.Int64(5),
						SehBucketLimit:                         aws.Int64(6),
						UncompressedSamplesLengthLimit:         aws.Int64(7),
						PublishingMetricsLimit:                 aws.Int64(8),
					},
				}, nil
			},
			expectedAgentConfig: AgentConfig{
				Status:      statusTerm,
				Limits:      defaultLimits,
				Definitions: DefaultDefinitions(),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider := NewCSMConfigProvider(&mockCSMService{
				GetPublishingConfigurationFn: c.ConfigFn,
				GetPublishingSchemaFn:        c.SchemaFn,
			}, c.interval)
			cfg1 := provider.RetrieveAgentConfig()
			time.Sleep(c.interval + 250*time.Millisecond)
			cfg2 := provider.RetrieveAgentConfig()

			if c.differentSchemaVersion && cfg1.SchemaVersion == cfg2.SchemaVersion {
				t.Errorf("expected different schema versions, but received %q", cfg1.SchemaVersion)
			}

			if !c.differentSchemaVersion && cfg1.SchemaVersion != cfg2.SchemaVersion {
				t.Errorf("expected the same schema versions, but received %q and %q", cfg1.SchemaVersion, cfg2.SchemaVersion)
			}

			if e, a := c.expectedAgentConfig, cfg2; !reflect.DeepEqual(e, a) {
				t.Errorf("expected %v, but received %v", e, a)
			}
			provider.Close()
		})
	}
}

type mockClient struct{}

func (m *mockClient) Write(metrics metametrics.Metrics) error {
	return nil
}

func init() {
	mock := &mockClient{}

	metametrics.MetricListener = metametrics.NewListenerAndStart(mock, 10, 1*time.Second)
}
