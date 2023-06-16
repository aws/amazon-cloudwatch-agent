// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"log"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/publisher"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"

	"github.com/aws/amazon-cloudwatch-agent/cfg/agentinfo"
	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	handlers "github.com/aws/amazon-cloudwatch-agent/handlers"
	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
)

const (
	defaultMaxDatumsPerCall               = 1000   // PutMetricData only supports up to 1000 data metrics per call by default
	defaultMaxValuesPerDatum              = 150    // By default only these number of values can be inserted into the value list
	bottomLinePayloadSizeInBytesToPublish = 999000 // 1MB payload size. Leave 1kb for the last datum buffer before applying compression ratio.
	metricChanBufferSize                  = 10000
	datumBatchChanBufferSize              = 50 // the number of requests we buffer
	maxConcurrentPublisher                = 10 // the number of CloudWatch clients send request concurrently
	pushIntervalInSec                     = 60 // 60 sec
	highResolutionTagKey                  = "aws:StorageResolution"
	defaultRetryCount                     = 5 // this is the retry count, the total attempts would be retry count + 1 at most.
	backoffRetryBase                      = 200 * time.Millisecond
	MaxDimensions                         = 30
)

const (
	opPutLogEvents       = "PutLogEvents"
	opPutMetricData      = "PutMetricData"
	dropOriginalWildcard = "*"
)

type CloudWatch struct {
	Region             string                   `toml:"region"`
	EndpointOverride   string                   `toml:"endpoint_override"`
	AccessKey          string                   `toml:"access_key"`
	SecretKey          string                   `toml:"secret_key"`
	RoleARN            string                   `toml:"role_arn"`
	Profile            string                   `toml:"profile"`
	Filename           string                   `toml:"shared_credential_file"`
	Token              string                   `toml:"token"`
	ForceFlushInterval internal.Duration        `toml:"force_flush_interval"` // unit is second
	MaxDatumsPerCall   int                      `toml:"max_datums_per_call"`
	MaxValuesPerDatum  int                      `toml:"max_values_per_datum"`
	MetricConfigs      []MetricDecorationConfig `toml:"metric_decoration"`
	RollupDimensions   [][]string               `toml:"rollup_dimensions"`
	DropOriginConfigs  map[string][]string      `toml:"drop_original_metrics"`
	Namespace          string                   `toml:"namespace"` // CloudWatch Metrics Namespace

	Log telegraf.Logger `toml:"-"`

	svc                    cloudwatchiface.CloudWatchAPI
	aggregator             Aggregator
	aggregatorShutdownChan chan struct{}
	aggregatorWaitGroup    sync.WaitGroup
	metricChan             chan telegraf.Metric
	datumBatchChan         chan []*cloudwatch.MetricDatum
	metricDatumBatch       *MetricDatumBatch
	shutdownChan           chan struct{}
	pushTicker             *time.Ticker
	metricDecorations      *MetricDecorations
	retries                int
	publisher              *publisher.Publisher
	retryer                *retryer.LogThrottleRetryer
	droppingOriginMetrics  map[string]map[string]struct{}
	agentInfo              agentinfo.AgentInfo
	lastRequestBytes       int
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

func (c *CloudWatch) SampleConfig() string {
	return sampleConfig
}

func (c *CloudWatch) Description() string {
	return "Configuration for AWS CloudWatch output."
}

func (c *CloudWatch) Connect() error {
	var err error
	c.agentInfo = agentinfo.New("")
	c.publisher, _ = publisher.NewPublisher(publisher.NewNonBlockingFifoQueue(metricChanBufferSize), maxConcurrentPublisher, 2*time.Second, c.WriteToCloudWatch)

	if c.metricDecorations, err = NewMetricDecorations(c.MetricConfigs); err != nil {
		return err
	}

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

	logThrottleRetryer := retryer.NewLogThrottleRetryer(c.Log)
	svc := cloudwatch.New(
		configProvider,
		&aws.Config{
			Endpoint: aws.String(c.EndpointOverride),
			Retryer:  logThrottleRetryer,
			LogLevel: configaws.SDKLogLevel(),
			Logger:   configaws.SDKLogger{},
		})

	svc.Handlers.Build.PushBackNamed(handlers.NewRequestCompressionHandler([]string{opPutLogEvents, opPutMetricData}))
	svc.Handlers.Build.PushBackNamed(handlers.NewCustomHeaderHandler("User-Agent", c.agentInfo.UserAgent()))
	svc.Handlers.Build.PushBackNamed(handlers.NewDynamicCustomHeaderHandler("X-Amz-Agent-Stats", c.agentInfo.StatsHeader))

	//Format unique roll up list
	c.RollupDimensions = GetUniqueRollupList(c.RollupDimensions)

	//Construct map for metrics that dropping origin
	c.droppingOriginMetrics = GetDroppingDimensionMap(c.DropOriginConfigs)

	c.svc = svc
	c.retryer = logThrottleRetryer
	c.startRoutines()
	return nil
}

func (c *CloudWatch) startRoutines() {
	c.metricChan = make(chan telegraf.Metric, metricChanBufferSize)
	c.datumBatchChan = make(chan []*cloudwatch.MetricDatum, datumBatchChanBufferSize)
	c.shutdownChan = make(chan struct{})
	c.aggregatorShutdownChan = make(chan struct{})
	c.aggregator = NewAggregator(c.metricChan, c.aggregatorShutdownChan, &c.aggregatorWaitGroup)
	if c.ForceFlushInterval.Duration == 0 {
		c.ForceFlushInterval.Duration = pushIntervalInSec * time.Second
	}
	if c.MaxDatumsPerCall == 0 {
		c.MaxDatumsPerCall = defaultMaxDatumsPerCall
	}
	if c.MaxValuesPerDatum == 0 {
		c.MaxValuesPerDatum = defaultMaxValuesPerDatum
	}
	setNewDistributionFunc(c.MaxValuesPerDatum)
	perRequestConstSize := overallConstPerRequestSize + len(c.Namespace) + namespaceOverheads
	c.metricDatumBatch = newMetricDatumBatch(c.MaxDatumsPerCall, perRequestConstSize)
	go c.pushMetricDatum()
	go c.publish()
}

func (c *CloudWatch) Close() error {
	log.Println("D! Stopping the CloudWatch output plugin")
	close(c.aggregatorShutdownChan)
	c.aggregatorWaitGroup.Wait()
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

func (c *CloudWatch) Write(metrics []telegraf.Metric) error {
	for _, m := range metrics {
		c.aggregator.AddMetric(m)
	}
	return nil
}

// Write data for a single point. A point can have many fields and one field
// is equal to one MetricDatum. There is a limit on how many MetricDatums a
// request can have so we process one Point at a time.
func (c *CloudWatch) pushMetricDatum() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case point := <-c.metricChan:
			datums := c.BuildMetricDatum(point)
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
	return len(b.Partition) > 0 && time.Now().Sub(b.BeginTime) >= c.ForceFlushInterval.Duration
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
// If thet batch buffer fills up the interval will be gradually reduced to avoid
// many agents bursting the backend.
func (c *CloudWatch) publish() {
	currentInterval := c.ForceFlushInterval.Duration
	nextMs := getFirstPushMs(currentInterval)
	bufferFullOccurred := false

	for {
		shouldPublish := false
		select {
		case <-c.shutdownChan:
			log.Printf("D! cloudwatch: publish routine receives the shutdown signal, exiting.")
			return
		case <-c.aggregatorShutdownChan:
			shouldPublish = true
		default:
		}

		nowMs := time.Now().UnixMilli()

		if c.metricDatumBatchFull() {
			if !bufferFullOccurred {
				// Set to true so this only happens once per push.
				bufferFullOccurred = true
				// Keep interval above above 1 second.
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
				currentInterval = c.ForceFlushInterval.Duration
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
		Namespace:  aws.String(c.Namespace),
	}
	var err error
	for i := 0; i < defaultRetryCount; i++ {
		startTime := time.Now()
		_, err = c.svc.PutMetricData(params)
		c.agentInfo.RecordOpData(time.Since(startTime), c.lastRequestBytes, err)
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

func (c *CloudWatch) decorateMetricName(category string, name string) (decoratedName string) {
	if c.metricDecorations != nil {
		decoratedName = c.metricDecorations.getRename(category, name)
	}
	if decoratedName == "" {
		if name == "value" {
			decoratedName = category
		} else {
			separator := "_"
			if runtime.GOOS == "windows" {
				separator = " "
			}
			decoratedName = strings.Join([]string{category, name}, separator)
		}
	}
	return
}

func (c *CloudWatch) decorateMetricUnit(category string, name string) (decoratedUnit string) {
	if c.metricDecorations != nil {
		decoratedUnit = c.metricDecorations.getUnit(category, name)
	}
	return
}

// Create MetricDatums according to metric roll up requirement for each field in a Point. Only fields with values that can be
// converted to float64 are supported. Non-supported fields are skipped.
func (c *CloudWatch) BuildMetricDatum(point telegraf.Metric) []*cloudwatch.MetricDatum {
	//high resolution logic
	isHighResolution := false
	highResolutionValue, ok := point.Tags()[highResolutionTagKey]
	if ok && strings.EqualFold(highResolutionValue, "true") {
		isHighResolution = true
		point.RemoveTag(highResolutionTagKey)
	}

	rawDimensions := BuildDimensions(point.Tags())
	dimensionsList := c.ProcessRollup(rawDimensions)
	//https://pratheekadidela.in/2016/02/11/is-append-in-go-efficient/
	//https://www.ardanlabs.com/blog/2013/08/understanding-slices-in-go-programming.html
	var datums []*cloudwatch.MetricDatum
	for k, v := range point.Fields() {
		var unit string
		var value float64
		var distList []distribution.Distribution

		switch t := v.(type) {
		case uint:
			value = float64(t)
		case uint8:
			value = float64(t)
		case uint16:
			value = float64(t)
		case uint32:
			value = float64(t)
		case uint64:
			value = float64(t)
		case int:
			value = float64(t)
		case int8:
			value = float64(t)
		case int16:
			value = float64(t)
		case int32:
			value = float64(t)
		case int64:
			value = float64(t)
		case float32:
			value = float64(t)
		case float64:
			value = t
		case bool:
			if t {
				value = 1
			} else {
				value = 0
			}
		case time.Time:
			value = float64(t.Unix())
		case distribution.Distribution:
			if t.Size() == 0 {
				// the distribution does not have a value
				continue
			}
			distList = resize(t, c.MaxValuesPerDatum)
			unit = t.Unit()
		default:
			// Skip unsupported type.
			continue
		}

		metricName := aws.String(c.decorateMetricName(point.Name(), k))
		if unit == "" {
			unit = c.decorateMetricUnit(point.Name(), k)
		}

		for index, dimensions := range dimensionsList {
			//index == 0 means it's the original metrics, and if the metric name and dimension matches, skip creating
			//metric datum
			if index == 0 && c.IsDropping(point.Name(), k) {
				continue
			}
			if len(distList) == 0 {
				datum := &cloudwatch.MetricDatum{
					MetricName: metricName,
					Dimensions: dimensions,
					Timestamp:  aws.Time(point.Time()),
					Value:      aws.Float64(value),
				}
				if unit != "" {
					datum.SetUnit(unit)
				}
				if isHighResolution {
					datum.SetStorageResolution(1)
				}
				datums = append(datums, datum)
			} else {
				for _, dist := range distList {
					datum := &cloudwatch.MetricDatum{
						MetricName: metricName,
						Dimensions: dimensions,
						Timestamp:  aws.Time(point.Time()),
					}
					values, counts := dist.ValuesAndCounts()
					datum.SetValues(aws.Float64Slice(values))
					datum.SetCounts(aws.Float64Slice(counts))
					datum.SetStatisticValues(&cloudwatch.StatisticSet{
						Maximum:     aws.Float64(dist.Maximum()),
						Minimum:     aws.Float64(dist.Minimum()),
						SampleCount: aws.Float64(dist.SampleCount()),
						Sum:         aws.Float64(dist.Sum()),
					})
					if unit != "" {
						datum.SetUnit(unit)
					}
					if isHighResolution {
						datum.SetStorageResolution(1)
					}
					datums = append(datums, datum)
				}
			}
		}
	}
	return datums
}

// Make a list of Dimensions by using a Point's tags. CloudWatch supports up to
// 30 dimensions per metric so we only keep up to the first 30 alphabetically.
// This always includes the "host" tag if it exists.
// See https://github.com/aws/amazon-cloudwatch-agent/issues/398
func BuildDimensions(mTags map[string]string) []*cloudwatch.Dimension {
	dimensions := make([]*cloudwatch.Dimension, 0, MaxDimensions)

	// This is pretty ugly but we always want to include the "host" tag if it exists.
	if host, ok := mTags["host"]; ok && host != "" {
		dimensions = append(dimensions, &cloudwatch.Dimension{
			Name:  aws.String("host"),
			Value: aws.String(host),
		})
	}

	var keys []string
	for k := range mTags {
		if k != "host" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, k := range keys {
		if len(dimensions) >= MaxDimensions {
			log.Printf("D! max MaxDimensions %v is less than than number of dimensions %v thus only taking the max number", MaxDimensions, len(dimensions))
			break
		}

		value := mTags[k]
		if value == "" {
			continue
		}

		dimensions = append(dimensions, &cloudwatch.Dimension{
			Name:  aws.String(k),
			Value: aws.String(mTags[k]),
		})
	}

	return dimensions
}

func (c *CloudWatch) ProcessRollup(rawDimension []*cloudwatch.Dimension) [][]*cloudwatch.Dimension {
	rawDimensionMap := map[string]string{}
	for _, v := range rawDimension {
		log.Printf("D! rawDimension: name: %s, values: %s\n", *v.Name, *v.Value)
		rawDimensionMap[*v.Name] = *v.Value
	}

	targetDimensionsList := c.RollupDimensions
	fullDimensionsList := [][]*cloudwatch.Dimension{rawDimension}

	for _, targetDimensions := range targetDimensionsList {
		i := 0
		extraDimensions := make([]*cloudwatch.Dimension, len(targetDimensions))
		for _, targetDimensionKey := range targetDimensions {
			if val, ok := rawDimensionMap[targetDimensionKey]; !ok {
				break
			} else {
				extraDimensions[i] = &cloudwatch.Dimension{
					Name:  aws.String(targetDimensionKey),
					Value: aws.String(val),
				}
			}
			i += 1
		}
		if i == len(targetDimensions) && !reflect.DeepEqual(rawDimension, extraDimensions) {
			fullDimensionsList = append(fullDimensionsList, extraDimensions)
		}

	}
	if len(fullDimensionsList) > 0 && len(fullDimensionsList[0]) > 0 {
		log.Printf("D! cloudwatch: Get Full dimensionList %v", fullDimensionsList)
	}
	return fullDimensionsList
}

func GetUniqueRollupList(inputLists [][]string) [][]string {
	uniqueLists := [][]string{}
	if len(inputLists) > 0 {
		uniqueLists = append(uniqueLists, inputLists[0])
	}
	for _, inputList := range inputLists {
		count := 0
		for _, u := range uniqueLists {
			if reflect.DeepEqual(inputList, u) {
				break
			}
			count += 1
			if count == len(uniqueLists) {
				uniqueLists = append(uniqueLists, inputList)
			}
		}
	}
	log.Printf("I! cloudwatch: get unique roll up list %v", uniqueLists)
	return uniqueLists
}

func (c *CloudWatch) IsDropping(metricName string, dimensionName string) bool {
	if droppingDimensions, ok := c.droppingOriginMetrics[metricName]; ok {
		if _, droppingAll := droppingDimensions[dropOriginalWildcard]; droppingAll {
			return true
		}
		_, dropping := droppingDimensions[dimensionName]
		return dropping
	}
	return false
}

func GetDroppingDimensionMap(input map[string][]string) map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{})
	for k, v := range input {
		result[k] = make(map[string]struct{})
		for _, dimension := range v {
			result[k][dimension] = struct{}{}
		}
	}
	return result
}

func init() {
	outputs.Add("cloudwatch", func() telegraf.Output {
		return &CloudWatch{}
	})
}
