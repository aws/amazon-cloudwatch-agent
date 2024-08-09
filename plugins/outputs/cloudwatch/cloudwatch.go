// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"context"
	"log"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/models"
	"github.com/influxdata/telegraf/plugins/outputs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/handlers"
	"github.com/aws/amazon-cloudwatch-agent/internal/publisher"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

const (
	defaultMaxDatumsPerCall               = 1000   // PutMetricData only supports up to 1000 data metrics per call by default
	defaultMaxValuesPerDatum              = 150    // By default only these number of values can be inserted into the value list
	bottomLinePayloadSizeInBytesToPublish = 999000 // 1MB payload size. Leave 1kb for the last datum buffer before applying compression ratio.
	metricChanBufferSize                  = 10000
	datumBatchChanBufferSize              = 50 // the number of requests we buffer
	maxConcurrentPublisher                = 10 // the number of CloudWatch clients send request concurrently
	defaultForceFlushInterval             = time.Minute
	highResolutionTagKey                  = "aws:StorageResolution"
	defaultRetryCount                     = 5 // this is the retry count, the total attempts would be retry count + 1 at most.
	backoffRetryBase                      = 200 * time.Millisecond
	MaxDimensions                         = 30
)

const (
	opPutLogEvents  = "PutLogEvents"
	opPutMetricData = "PutMetricData"
)

type CloudWatch struct {
	config *Config
	logger *zap.Logger
	svc    cloudwatchiface.CloudWatchAPI
	// todo: may want to increase the size of the chan since the type changed.
	// 1 telegraf Metric could have many Fields.
	// Each field corresponds to a MetricDatum.
	metricChan             chan *aggregationDatum
	datumBatchChan         chan []*cloudwatch.MetricDatum
	metricDatumBatch       *MetricDatumBatch
	shutdownChan           chan struct{}
	retries                int
	publisher              *publisher.Publisher
	retryer                *retryer.LogThrottleRetryer
	droppingOriginMetrics  collections.Set[string]
	aggregator             Aggregator
	aggregatorShutdownChan chan struct{}
	aggregatorWaitGroup    sync.WaitGroup
	lastRequestBytes       int
}

// Compile time interface check.
var _ exporter.Metrics = (*CloudWatch)(nil)

func (c *CloudWatch) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *CloudWatch) Start(_ context.Context, host component.Host) error {
	c.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(metricChanBufferSize),
		maxConcurrentPublisher,
		2*time.Second,
		c.WriteToCloudWatch)
	credentialConfig := &configaws.CredentialConfig{
		Region:    c.config.Region,
		AccessKey: c.config.AccessKey,
		SecretKey: c.config.SecretKey,
		RoleARN:   c.config.RoleARN,
		Profile:   c.config.Profile,
		Filename:  c.config.SharedCredentialFilename,
		Token:     c.config.Token,
	}
	configProvider := credentialConfig.Credentials()
	logger := models.NewLogger("outputs", "cloudwatch", "")
	logThrottleRetryer := retryer.NewLogThrottleRetryer(logger)
	svc := cloudwatch.New(
		configProvider,
		&aws.Config{
			Endpoint: aws.String(c.config.EndpointOverride),
			Retryer:  logThrottleRetryer,
			LogLevel: configaws.SDKLogLevel(),
			Logger:   configaws.SDKLogger{},
		})
	svc.Handlers.Build.PushBackNamed(handlers.NewRequestCompressionHandler([]string{opPutLogEvents, opPutMetricData}))
	if c.config.MiddlewareID != nil {
		awsmiddleware.TryConfigure(c.logger, host, *c.config.MiddlewareID, awsmiddleware.SDKv1(&svc.Handlers))
	}
	//Format unique roll up list
	c.config.RollupDimensions = GetUniqueRollupList(c.config.RollupDimensions)
	c.svc = svc
	c.retryer = logThrottleRetryer
	c.startRoutines()
	return nil
}

func (c *CloudWatch) startRoutines() {
	setNewDistributionFunc(c.config.MaxValuesPerDatum)
	c.metricChan = make(chan *aggregationDatum, metricChanBufferSize)
	c.datumBatchChan = make(chan []*cloudwatch.MetricDatum, datumBatchChanBufferSize)
	c.shutdownChan = make(chan struct{})
	c.aggregatorShutdownChan = make(chan struct{})
	c.aggregator = NewAggregator(c.metricChan, c.aggregatorShutdownChan, &c.aggregatorWaitGroup)
	perRequestConstSize := overallConstPerRequestSize + len(c.config.Namespace) + namespaceOverheads
	c.metricDatumBatch = newMetricDatumBatch(c.config.MaxDatumsPerCall, perRequestConstSize)
	go c.pushMetricDatum()
	go c.publish()
}

func (c *CloudWatch) Shutdown(ctx context.Context) error {
	log.Println("D! Stopping the CloudWatch output plugin")
	for i := 0; i < 5; i++ {
		if len(c.metricChan) == 0 && len(c.datumBatchChan) == 0 {
			break
		} else {
			log.Printf("D! CloudWatch Close, %vth time to sleep since there is still some metric data remaining to publish.", i)
			time.Sleep(time.Second)
		}
	}
	if metricChanLen, datumBatchChanLen := len(c.metricChan), len(c.datumBatchChan); metricChanLen != 0 || datumBatchChanLen != 0 {
		log.Printf("D! CloudWatch Close, metricChan length = %v, datumBatchChan length = %v.", metricChanLen, datumBatchChanLen)
	}
	close(c.shutdownChan)
	c.publisher.Close()
	c.retryer.Stop()
	log.Println("D! Stopped the CloudWatch output plugin")
	return nil
}

// ConsumeMetrics queues metrics to be published to CW.
// The actual publishing will occur in a long running goroutine.
// This method can block when publishing is backed up.
func (c *CloudWatch) ConsumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	datums := ConvertOtelMetrics(metrics)
	for _, d := range datums {
		c.aggregator.AddMetric(d)
	}
	return nil
}

// pushMetricDatum groups datums into batches for efficient API calls.
// When a batch is full it is queued up for sending.
// Even if the batch is not full it will still get sent after the flush interval.
func (c *CloudWatch) pushMetricDatum() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case metric := <-c.metricChan:
			datums := c.BuildMetricDatum(metric)
			numberOfPartitions := len(datums)
			for i := 0; i < numberOfPartitions; i++ {
				c.metricDatumBatch.Partition = append(c.metricDatumBatch.Partition, datums[i])
				c.metricDatumBatch.Size += payload(datums[i])
				if c.metricDatumBatch.isFull() {
					// if batch is full
					c.datumBatchChan <- c.metricDatumBatch.Partition
					c.metricDatumBatch.clear()
				}
			}
		case <-ticker.C:
			if c.timeToPublish(c.metricDatumBatch) {
				// if the time to publish comes
				c.lastRequestBytes = c.metricDatumBatch.Size
				c.datumBatchChan <- c.metricDatumBatch.Partition
				c.metricDatumBatch.clear()
			}
		case <-c.shutdownChan:
			return
		}
	}
}

type MetricDatumBatch struct {
	MaxDatumsPerCall    int
	Partition           []*cloudwatch.MetricDatum
	BeginTime           time.Time
	Size                int
	perRequestConstSize int
}

func newMetricDatumBatch(maxDatumsPerCall, perRequestConstSize int) *MetricDatumBatch {
	return &MetricDatumBatch{
		MaxDatumsPerCall:    maxDatumsPerCall,
		Partition:           make([]*cloudwatch.MetricDatum, 0, maxDatumsPerCall),
		BeginTime:           time.Now(),
		Size:                perRequestConstSize,
		perRequestConstSize: perRequestConstSize,
	}
}

func (b *MetricDatumBatch) clear() {
	b.Partition = make([]*cloudwatch.MetricDatum, 0, b.MaxDatumsPerCall)
	b.BeginTime = time.Now()
	b.Size = b.perRequestConstSize
}

func (b *MetricDatumBatch) isFull() bool {
	return len(b.Partition) >= b.MaxDatumsPerCall || b.Size >= bottomLinePayloadSizeInBytesToPublish
}

func (c *CloudWatch) timeToPublish(b *MetricDatumBatch) bool {
	return len(b.Partition) > 0 && time.Since(b.BeginTime) >= c.config.ForceFlushInterval
}

// getFirstPushMs returns the time at which the first upload should occur.
// It uses random jitter as an offset from the start of the given interval.
func getFirstPushMs(interval time.Duration) int64 {
	publishJitter := publishJitter(interval)
	log.Printf("I! cloudwatch: publish with ForceFlushInterval: %v, Publish Jitter: %v",
		interval, publishJitter)
	nowMs := time.Now().UnixMilli()
	// Truncate i.e. round down, then add jitter.
	// If the rounded down time is in the past, move it forward.
	nextMs := nowMs - (nowMs % interval.Milliseconds()) + publishJitter.Milliseconds()
	if nextMs < nowMs {
		nextMs += interval.Milliseconds()
	}
	return nextMs
}

// publish loops until a shutdown occurs.
// It periodically tries pushing batches of metrics (if there are any).
// If the batch buffer fills up the interval will be gradually reduced to avoid
// many agents bursting the backend.
func (c *CloudWatch) publish() {
	currentInterval := c.config.ForceFlushInterval
	nextMs := getFirstPushMs(currentInterval)
	bufferFullOccurred := false

	for {
		shouldPublish := false
		select {
		case <-c.shutdownChan:
			log.Printf("D! cloudwatch: publish routine receives the shutdown signal, exiting.")
			return
		default:
		}

		nowMs := time.Now().UnixMilli()

		if c.metricDatumBatchFull() {
			if !bufferFullOccurred {
				// Set to true so this only happens once per push.
				bufferFullOccurred = true
				// Keep interval above 1 second.
				if currentInterval.Seconds() > 1 {
					currentInterval /= 2
					if currentInterval.Seconds() < 1 {
						currentInterval = 1 * time.Second
					}
					// Cut the remaining interval in half.
					nextMs = nowMs + ((nextMs - nowMs) / 2)
				}
			}
		}

		if nowMs >= nextMs {
			shouldPublish = true
			// Restore interval if buffer did not fill up during this interval.
			if !bufferFullOccurred {
				currentInterval = c.config.ForceFlushInterval
			}
			nextMs += currentInterval.Milliseconds()
		}

		if shouldPublish {
			c.pushMetricDatumBatch()
			bufferFullOccurred = false
		}
		// Sleep 1 second, unless the nextMs is less than a second away.
		if nextMs-nowMs > time.Second.Milliseconds() {
			time.Sleep(time.Second)
		} else {
			time.Sleep(time.Duration(nextMs-nowMs) * time.Millisecond)
		}
	}
}

// metricDatumBatchFull returns true if the channel/buffer of batches if full.
func (c *CloudWatch) metricDatumBatchFull() bool {
	return len(c.datumBatchChan) >= datumBatchChanBufferSize
}

// pushMetricDatumBatch will try receiving on the channel, and if successful,
// then it publishes the received batch.
func (c *CloudWatch) pushMetricDatumBatch() {
	for {
		select {
		case datumBatch := <-c.datumBatchChan:
			c.publisher.Publish(datumBatch)
			continue
		default:
		}
		break
	}
}

// backoffSleep sleeps some amount of time based on number of retries done.
func (c *CloudWatch) backoffSleep() {
	d := 1 * time.Minute
	if c.retries <= defaultRetryCount {
		d = backoffRetryBase * time.Duration(1<<c.retries)
	}
	d = (d / 2) + publishJitter(d/2)
	log.Printf("W! cloudwatch: %v retries, going to sleep %v ms before retrying.",
		c.retries, d.Milliseconds())
	c.retries++
	time.Sleep(d)
}

func (c *CloudWatch) WriteToCloudWatch(req interface{}) {
	datums := req.([]*cloudwatch.MetricDatum)
	params := &cloudwatch.PutMetricDataInput{
		MetricData: datums,
		Namespace:  aws.String(c.config.Namespace),
	}
	var err error
	for i := 0; i < defaultRetryCount; i++ {
		_, err = c.svc.PutMetricData(params)
		if err != nil {
			awsErr, ok := err.(awserr.Error)
			if !ok {
				log.Printf("E! cloudwatch: Cannot cast PutMetricData error %v into awserr.Error.", err)
				c.backoffSleep()
				continue
			}
			switch awsErr.Code() {
			case cloudwatch.ErrCodeLimitExceededFault, cloudwatch.ErrCodeInternalServiceFault:
				log.Printf("W! cloudwatch: PutMetricData, error: %s, message: %s",
					awsErr.Code(),
					awsErr.Message())
				c.backoffSleep()
				continue

			default:
				log.Printf("E! cloudwatch: code: %s, message: %s, original error: %+v", awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
				c.backoffSleep()
			}
		} else {
			c.retries = 0
		}
		break
	}
	if err != nil {
		log.Println("E! cloudwatch: WriteToCloudWatch failure, err: ", err)
	}
}

// BuildMetricDatum may just return the datum as-is.
// Or it might expand it into many datums due to dimension aggregation.
// There may also be more datums due to resize() on a distribution.
func (c *CloudWatch) BuildMetricDatum(metric *aggregationDatum) []*cloudwatch.MetricDatum {
	var datums []*cloudwatch.MetricDatum
	var distList []distribution.Distribution

	if metric.distribution != nil {
		if metric.distribution.Size() == 0 {
			log.Printf("E! metric has a distribution with no entries, %s", *metric.MetricName)
			return datums
		}
		if metric.distribution.Unit() != "" {
			metric.SetUnit(metric.distribution.Unit())
		}
		distList = resize(metric.distribution, c.config.MaxValuesPerDatum)
	}

	dimensionsList := c.ProcessRollup(metric.Dimensions)
	for index, dimensions := range dimensionsList {
		//index == 0 means it's the original metrics, and if the metric name and dimension matches, skip creating
		//metric datum
		if index == 0 && c.IsDropping(*metric.MetricDatum.MetricName) {
			continue
		}
		if len(distList) == 0 {
			if !distribution.IsSupportedValue(*metric.Value, distribution.MinValue, distribution.MaxValue) {
				log.Printf("E! metric (%s) has an unsupported value: %v, dropping it", *metric.MetricName, *metric.Value)
				continue
			}
			// Not a distribution.
			datum := &cloudwatch.MetricDatum{
				MetricName:        metric.MetricName,
				Dimensions:        dimensions,
				Timestamp:         metric.Timestamp,
				Unit:              metric.Unit,
				StorageResolution: metric.StorageResolution,
				Value:             metric.Value,
			}
			datums = append(datums, datum)
		} else {
			for _, dist := range distList {
				values, counts := dist.ValuesAndCounts()
				s := cloudwatch.StatisticSet{}
				s.SetMaximum(dist.Maximum())
				s.SetMinimum(dist.Minimum())
				s.SetSampleCount(dist.SampleCount())
				s.SetSum(dist.Sum())
				// Beware there may be many datums sharing pointers to the same
				// strings for metric names, dimensions, etc.
				// It is fine since at this point the values will not change.
				datum := &cloudwatch.MetricDatum{
					MetricName:        metric.MetricName,
					Dimensions:        dimensions,
					Timestamp:         metric.Timestamp,
					Unit:              metric.Unit,
					StorageResolution: metric.StorageResolution,
					Values:            aws.Float64Slice(values),
					Counts:            aws.Float64Slice(counts),
					StatisticValues:   &s,
				}
				datums = append(datums, datum)
			}
		}
	}
	return datums
}

func (c *CloudWatch) IsDropping(metricName string) bool {
	// Check if any metrics are provided in drop_original_metrics
	if len(c.config.DropOriginalConfigs) == 0 {
		return false
	}
	if _, ok := c.config.DropOriginalConfigs[metricName]; ok {
		return true
	}
	return false
}

// sortedTagKeys returns a sorted list of keys in the map.
// Necessary for comparing a metric-name and its dimensions to determine
// if 2 metrics are actually the same.
func sortedTagKeys(tagMap map[string]string) []string {
	// Allocate slice with proper size and avoid append.
	keys := make([]string, 0, len(tagMap))
	for k := range tagMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// BuildDimensions converts the given map of strings to a list of dimensions.
// CloudWatch supports up to 30 dimensions per metric.
// So keep up to the first 30 alphabetically.
// This always includes the "host" tag if it exists.
// See https://github.com/aws/amazon-cloudwatch-agent/issues/398
func BuildDimensions(tagMap map[string]string) []*cloudwatch.Dimension {
	if len(tagMap) > MaxDimensions {
		log.Printf("D! cloudwatch: dropping dimensions, max %v, count %v",
			MaxDimensions, len(tagMap))
	}
	dimensions := make([]*cloudwatch.Dimension, 0, MaxDimensions)
	// This is pretty ugly but we always want to include the "host" tag if it exists.
	if host, ok := tagMap["host"]; ok && host != "" {
		dimensions = append(dimensions, &cloudwatch.Dimension{
			Name:  aws.String("host"),
			Value: aws.String(host),
		})
	}
	sortedKeys := sortedTagKeys(tagMap)
	for _, k := range sortedKeys {
		if len(dimensions) >= MaxDimensions {
			break
		}
		if k == "host" {
			continue
		}
		value := tagMap[k]
		if value == "" {
			continue
		}
		dimensions = append(dimensions, &cloudwatch.Dimension{
			Name:  aws.String(k),
			Value: aws.String(tagMap[k]),
		})
	}
	return dimensions
}

// ProcessRollup creates the dimension sets based on the dimensions available in the original metric.
func (c *CloudWatch) ProcessRollup(rawDimensions []*cloudwatch.Dimension) [][]*cloudwatch.Dimension {
	rawDimensionMap := map[string]string{}
	for _, v := range rawDimensions {
		rawDimensionMap[*v.Name] = *v.Value
	}
	targetDimensionsList := c.config.RollupDimensions
	fullDimensionsList := [][]*cloudwatch.Dimension{rawDimensions}
	for _, targetDimensions := range targetDimensionsList {
		// skip if target dimensions count is same or more than the original metric.
		// cannot have dimensions that do not exist in the original metric.
		if len(targetDimensions) >= len(rawDimensions) {
			continue
		}
		count := 0
		extraDimensions := make([]*cloudwatch.Dimension, len(targetDimensions))
		for _, targetDimensionKey := range targetDimensions {
			if val, ok := rawDimensionMap[targetDimensionKey]; !ok {
				break
			} else {
				extraDimensions[count] = &cloudwatch.Dimension{
					Name:  aws.String(targetDimensionKey),
					Value: aws.String(val),
				}
			}
			count++
		}
		if count == len(targetDimensions) {
			fullDimensionsList = append(fullDimensionsList, extraDimensions)
		}
	}
	return fullDimensionsList
}

// GetUniqueRollupList filters out duplicate dimensions within the sets and filters
// duplicate sets.
func GetUniqueRollupList(inputLists [][]string) [][]string {
	var uniqueSets []collections.Set[string]
	for _, inputList := range inputLists {
		inputSet := collections.NewSet(inputList...)
		count := 0
		for _, uniqueSet := range uniqueSets {
			if reflect.DeepEqual(inputSet, uniqueSet) {
				break
			}
			count++
		}
		if count == len(uniqueSets) {
			uniqueSets = append(uniqueSets, inputSet)
		}
	}
	uniqueLists := make([][]string, len(uniqueSets))
	for i, uniqueSet := range uniqueSets {
		uniqueLists[i] = maps.Keys(uniqueSet)
		sort.Strings(uniqueLists[i])
	}
	log.Printf("I! cloudwatch: get unique roll up list %v", uniqueLists)
	return uniqueLists
}

func (c *CloudWatch) SampleConfig() string {
	return ""
}

func (c *CloudWatch) Description() string {
	return "Configuration for AWS CloudWatch output."
}

func (c *CloudWatch) Connect() error {
	return nil
}

func (c *CloudWatch) Close() error {
	return nil
}

func (c *CloudWatch) Write(metrics []telegraf.Metric) error {
	return nil
}

func init() {
	outputs.Add("cloudwatch", func() telegraf.Output {
		return &CloudWatch{}
	})
}
