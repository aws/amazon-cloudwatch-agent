// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"runtime"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm"
	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane"
	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane/sdkmetricsdataplaneiface"
	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/influxdata/telegraf"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
	"github.com/aws/amazon-cloudwatch-agent/handlers"
	"github.com/aws/amazon-cloudwatch-agent/internal/models"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/metametrics"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/providers"
	"github.com/influxdata/telegraf/plugins/outputs"
)

const (
	version           = "1.0"
	retryRecordStatus = "ERROR"

	defaultRecordLimit = 5
	tagName            = "awscsm"

	maxQueueBacklogSize = 5000
)

var (
	ec2Env = []*sdkmetricsdataplane.EnvironmentProperty{
		{
			EnvironmentPropertyTag: aws.String(sdkmetricsdataplane.EnvironmentPropertyTagEc2),
		},
	}
)

// CSM structure houses the configuration retrieved from the configuration file.
// This also contains the sdkmetrics dataplane client which will be used to put aggregated metrics
// to.
type CSM struct {
	Region          string `toml:"region"`
	AccessKey       string `toml:"access_key"`
	SecretKey       string `toml:"secret_key"`
	RoleARN         string `toml:"role_arn"`
	Profile         string `toml:"profile"`
	Filename        string `toml:"shared_credential_file"`
	Token           string `toml:"token"`
	MemoryLimitInMb int    `toml:"memory_limit_in_mb"`
	LogLevel        int    `toml:"log_level"`

	EndpointOverride string `toml:"endpoint_override"`

	instanceMetadata *ec2metadata.EC2InstanceIdentityDocument

	queueCh          chan awscsmmetrics.Metric
	publishingOffset time.Duration
	sendRecordLimit  int

	cachedAgentConfig providers.AgentConfig
	dataplaneClient   sdkmetricsdataplaneiface.SDKMetricsDataplaneAPI

	logger loggeriface
}

var sampleConfig = `
  ## Amazon REGION
  region = "us-east-1"

  ## Amazon Credentials
  ## Credentials are loaded in the following order
  ## 1) Assumed credentials via STS if role_arn is specified
  ## 2) explicit credentials from 'access_key' and 'secret_key'
  ## 3) shared profile from 'profile'
  ## 4) environment variables
  ## 5) shared credentials file
  ## 6) EC2 Instance Profile
  #access_key = ""
  #secret_key = ""
  #token = ""
  #role_arn = ""
  #profile = ""
  #shared_credential_file = ""

  ## Namespace for the CloudWatch MetricDatums
  namespace = "InfluxData/Telegraf"

  ## RollupDimensions
  # RollupDimensions = [["host"],["host", "ImageId"],[]]
`

// SampleConfig returns the sample config
func (c *CSM) SampleConfig() string {
	return sampleConfig
}

// Description will return the description of the CSM output plugin
func (c *CSM) Description() string {
	return "Configuration for CSM output."
}

// Connect will bootstrap the client and add the user agent handler.
func (c *CSM) Connect() error {
	c.logger = newLogger(c.LogLevel)

	credentialConfig := &configaws.CredentialConfig{
		Region:    c.Region,
		AccessKey: c.AccessKey,
		SecretKey: c.SecretKey,
		RoleARN:   c.RoleARN,
		Profile:   c.Profile,
		Filename:  c.Filename,
		Token:     c.Token,
	}

	configProvider := credentialConfig.Credentials()

	metadataClient := ec2metadata.New(configProvider)
	instanceMetadata, err := metadataClient.GetInstanceIdentityDocument()
	region := c.Region

	if err == nil {
		c.instanceMetadata = &instanceMetadata
		if region == "" {
			region = c.instanceMetadata.Region
		}
		c.logger.Log("EC2Metadata found")
	}

	c.logger.Log("Using region " + region)

	credentialConfig.Region = region

	c.queueCh = models.AwsCsmOutputChannel
	c.publishingOffset = time.Duration(rand.Int63n(int64(60 * time.Second)))

	commonCreds := credentialConfig.Credentials()
	commonCfg := commonCreds.ClientConfig(csm.ServiceName, &aws.Config{
		LogLevel: configaws.SDKLogLevel(),
		Logger:   configaws.SDKLogger{},
	})

	if c.LogLevel > 0 {
		commonCfg.Config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	commonCfg.Config.Logger = aws.LoggerFunc(func(args ...interface{}) {
		log.Println(args...)
	})

	commonSession := session.New(commonCfg.Config)

	endpoint := fmt.Sprintf("https://control.sdkmetrics.%s.amazonaws.com", region)
	if len(c.EndpointOverride) > 0 {
		endpoint = c.EndpointOverride
	}

	controlPlaneConfigOverride := aws.Config{
		Endpoint: aws.String(endpoint),
		LogLevel: configaws.SDKLogLevel(),
		Logger:   configaws.SDKLogger{},
	}

	controlplane := csm.New(commonSession, &controlPlaneConfigOverride)
	//TODO: work on this when we have a proper versioning mechanism
	//TODO: we need to find a way to expose enabled plugins
	//TODO: custom metrics adoption rate detection and be able to monitor any plugin enable rate
	userAgent := fmt.Sprintf("%s/%s (%s; %s; %s) %s", "CWAgent/CSM", "1.0", runtime.Version(), runtime.GOOS, runtime.GOARCH, "list of enabled input/output plugins")
	controlplane.Handlers.Build.PushBackNamed(handlers.NewCustomHeaderHandler("User-Agent", userAgent))

	providers.Config = providers.NewCSMConfigProvider(controlplane, providers.DefaultInterval)
	c.sendRecordLimit = providers.Config.RetrieveAgentConfig().Limits.MaxRecords

	os := runtime.GOOS
	env := csm.HostEnvironment{
		Os: &os,
	}

	if c.instanceMetadata != nil {
		env.AvailabilityZone = &c.instanceMetadata.AvailabilityZone
		env.InstanceId = &c.instanceMetadata.InstanceID
		env.Properties = []*string{aws.String(sdkmetricsdataplane.EnvironmentPropertyTagEc2)}
	}

	writer := NewCSMWriter(controlplane, env)
	metametrics.MetricListener = metametrics.NewListenerAndStart(writer, 1000, 5*time.Minute)

	dataPlaneConfigOverride := aws.Config{
		MaxRetries: aws.Int(0),
		LogLevel:   configaws.SDKLogLevel(),
		Logger:     configaws.SDKLogger{},
	}

	dataplaneClient := sdkmetricsdataplane.New(commonSession, &dataPlaneConfigOverride)
	dataplaneClient.Handlers.Build.PushBackNamed(handlers.NewCustomHeaderHandler("User-Agent", userAgent))
	c.dataplaneClient = dataplaneClient

	go c.publishJob()

	return nil
}

// Close will flush remaining metrics in the queue
func (c *CSM) Close() error {
	return nil
}

// Write ...
func (c *CSM) Write(ms []telegraf.Metric) error {
	return nil
}

func (c *CSM) publishJob() {
	queue := []awscsmmetrics.Metric{}
	ring := newRecordRing(int64(c.MemoryLimitInMb) * 1024 * 1024)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	// Sleeping here to prevent the agent from hitting the
	// service all at once if multiple instance of the agent
	// is running.
	time.Sleep(c.publishingOffset)
	for {
		select {
		case m := <-c.queueCh:
			// ignore any metrics coming in if the queue limit
			// has been reached
			if len(queue) >= maxQueueBacklogSize {
				c.logger.Log(len(queue), "metrics dropped")
				break
			}

			queue = append(queue, m)
		case <-ticker.C:
			c.logger.Log("metrics queued", len(queue))
			for _, v := range queue {
				m, err := c.adaptMetricData(v, c.logger)
				// m will be nil if an error was encountered during UID
				// and we do not not want to continue appending that metric to the list
				if err != nil {
					c.logger.Log("E!", err)
					continue
				}

				ring.pushFront(m)
			}

			queue = queue[0:0]
			c.cachedAgentConfig = providers.Config.RetrieveAgentConfig()

			if !ring.empty() && c.cachedAgentConfig.CanPublish() {
				c.publish(&ring)
			}
		}
	}
}

func (c *CSM) updateSendRecordLimit(succeeded, empty bool) {
	if empty {
		return
	}

	if succeeded {
		cfg := providers.Config.RetrieveAgentConfig()
		c.sendRecordLimit += cfg.Limits.MaxRecords
	} else {
		// split sendRecordLimit in half
		c.sendRecordLimit >>= 1
		if c.sendRecordLimit == 0 {
			c.sendRecordLimit = 1
		}
	}
}

const (
	opPutRecordsKey = "PutRecords"
)

// publish will setup the inputs for PutRecords and make the API call
// on the sdkmetrics dataplane client. This will publish only 5 records at most at
// a time. If there are more than 5 records, it will make API calls per 5
// records until all records have been submitted.
func (c *CSM) publish(records *recordRing) {
	retryableMetrics := []*sdkmetricsdataplane.SdkMonitoringRecord{}
	errOccurred := false

	sent := 0
	limit := c.sendRecordLimit

	concreteClient, ok := c.dataplaneClient.(*sdkmetricsdataplane.SDKMetricsDataplane)
	if ok {
		// The v1 SDK of Go does not support endpoint resolvers in quite the way we need (auto fill
		// in the Endpoint values as each request is called) and so, since our requests are all
		// made sequentially in the same goroutine, it's safe to actually set the two Endpoint
		// fields used by the Go SDK's request processing.
		//
		// There's a good chance that the necessary endpoint resolver functionality will get
		// pulled into the v1 SDK, and so when that happens, we should switch to a resolver
		// rather than doing these raw assignments.
		currentConfig := providers.Config.RetrieveAgentConfig()
		url := url.URL{
			Host:   currentConfig.Endpoint,
			Scheme: "https",
		}

		endpoint := url.String()
		concreteClient.ClientInfo.Endpoint = endpoint
		concreteClient.Config.Endpoint = &endpoint
	}

	for !records.empty() && sent < limit {
		os := runtime.GOOS
		env := &sdkmetricsdataplane.HostEnvironment{
			Os: &os,
		}

		if c.instanceMetadata != nil {
			env.AvailabilityZone = &c.instanceMetadata.AvailabilityZone
			env.InstanceId = &c.instanceMetadata.InstanceID
			env.Properties = ec2Env
		}

		input := &sdkmetricsdataplane.PutRecordsInput{
			Environment: env,
			SdkRecords:  []*sdkmetricsdataplane.SdkMonitoringRecord{},
		}

		recordCount := 0
		for !records.empty() && recordCount < defaultRecordLimit {
			input.SdkRecords = append(input.SdkRecords, records.popFront())
			recordCount++
		}

		apiCallTimestamp := time.Now()

		resp, err := c.dataplaneClient.PutRecords(input)
		metametrics.MetricListener.CountSuccess(opPutRecordsKey, err == nil, apiCallTimestamp, providers.Config.RetrieveAgentConfig().Endpoint)

		if err != nil {
			log.Println("E! failed to put record")
			c.logger.Log("-------- Put Record Error --------\n", err.Error())

			errOccurred = true
			if retryRecordsOnError(err) {
				retryableMetrics = append(retryableMetrics, input.SdkRecords...)
			}

			break
		}

		c.logger.Log("Failed sending", failedAmount(resp.Statuses), "records")
		retryableMetrics = append(retryableMetrics, retryRecords(input.SdkRecords, resp.Statuses)...)
		sent += recordCount
	}

	// Adding back in reverse order restores the original order they were pulled out in
	// This in turn preserves the (roughly) timestamp-sorted order of the record ring
	for ri := len(retryableMetrics) - 1; ri >= 0; ri-- {
		records.pushFront(retryableMetrics[ri])
	}

	c.logger.Log("Successfully sent", sent-len(retryableMetrics), "records")
	c.logger.Log("Retrying", len(retryableMetrics), "records")

	// update the send record limit based off how many records are left to publish
	c.updateSendRecordLimit(!errOccurred, records.empty())
}

func (c *CSM) adaptMetricData(m awscsmmetrics.Metric, logger loggeriface) (*sdkmetricsdataplane.SdkMonitoringRecord, error) {
	input := &sdkmetricsdataplane.SdkMonitoringRecord{}
	ks := m.GetKeys()
	keys := []*sdkmetricsdataplane.SdkAggregationKeyEntry{}
	for dimensionKey, dimensionValue := range ks {
		keys = append(keys, &sdkmetricsdataplane.SdkAggregationKeyEntry{
			Key:   aws.String(dimensionKey),
			Value: aws.String(dimensionValue),
		})
	}

	input.FrequencyMetrics = adaptToCSMFrequencyMetrics(m.GetFrequencyMetrics())
	input.SehMetrics = adaptToCSMSEHMetrics(m.GetSEHMetrics())
	input.AggregationKey = &sdkmetricsdataplane.SdkAggregationKey{}
	input.AggregationKey.Timestamp = aws.Time(m.GetTimestamp())
	input.AggregationKey.Keys = keys
	input.Version = aws.String(version)

	samples := m.GetSamples()
	if len(samples) > 0 {
		c.logger.Log("adding", len(samples), "samples")
		if compressed, checksum, uncompressedLength, err := compressSamples(samples); err == nil {
			input.CompressedEventSamples = &compressed
			input.UncompressedSamplesChecksum = &checksum
			input.UncompressedSamplesLength = &uncompressedLength
		} else {
			// TODO: Figure out what we should do with these samples if an error is
			// returned

			// this error shouldn't prevent other metrics to be logged hence why it
			// is not returned
			logger.Log("E! failed compressing samples", err)
		}
	}

	uid, err := generateUID(m)
	if err != nil {
		return nil, err
	}

	input.Id = &uid
	return input, nil
}

func init() {
	outputs.Add("aws_csm", func() telegraf.Output {
		return &CSM{}
	})
}
