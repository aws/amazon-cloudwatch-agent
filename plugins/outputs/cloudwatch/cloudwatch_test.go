// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"context"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/publisher"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

// Return true if found.
func contains(dimensions []*cloudwatch.Dimension, key string, val string) bool {
	for _, d := range dimensions {
		if *d.Name == key && *d.Value == val {
			return true
		}
	}
	return false
}

// Test that each tag becomes one dimension.
// Test that no more than 30 dimensions will get returned.
// Test that if "host" dimension exists, it is always included.
func TestBuildDimensions(t *testing.T) {
	assert := assert.New(t)
	// nil
	dims := BuildDimensions(nil)
	assert.Equal(0, len(dims))
	// empty
	dims = BuildDimensions(make(map[string]string))
	assert.Equal(0, len(dims))
	// Always expect "host". Expect no more than 30.
	for i := 1; i < 40; i++ {
		tags := make(map[string]string, i)
		for j := 0; j < i; j++ {
			key := "key" + strconv.Itoa(j)
			val := "val" + strconv.Itoa(j)
			tags[key] = val
		}
		expectedLen := i
		// Test with and without host
		if i%2 == 0 {
			tags["host"] = "valhost"
			expectedLen++
		}
		if expectedLen > 30 {
			expectedLen = 30
		}
		dims = BuildDimensions(tags)
		hostCount := 0
		keyCount := 0
		valCount := 0
		for _, d := range dims {
			if strings.HasPrefix(*d.Name, "host") {
				hostCount++
			}
			if strings.HasPrefix(*d.Name, "key") {
				keyCount++
			}
			if strings.HasPrefix(*d.Value, "val") {
				valCount++
			}
		}

		assert.Equal(expectedLen, valCount)
		if i%2 == 0 {
			assert.Equal(1, hostCount)
			assert.Equal(expectedLen-1, keyCount)
		} else {
			assert.Equal(0, hostCount)
			assert.Equal(expectedLen, keyCount)
		}

	}
}

func TestProcessRollup(t *testing.T) {
	svc := new(mockCloudWatchClient)
	cw := newCloudWatchClient(svc, time.Second)
	cw.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(10),
		10,
		2*time.Second,
		cw.WriteToCloudWatch,
	)

	testRawDimensions := []*cloudwatch.Dimension{
		{
			Name:  aws.String("d1"),
			Value: aws.String("v1"),
		},
		{
			Name:  aws.String("d2"),
			Value: aws.String("v2"),
		},
		{
			Name:  aws.String("d3"),
			Value: aws.String("v3"),
		},
	}

	testCases := map[string]struct {
		rollupDimensions [][]string
		rawDimensions    []*cloudwatch.Dimension
		want             [][]*cloudwatch.Dimension
	}{
		"WithSimpleRollup": {
			rollupDimensions: [][]string{{"d1", "d2"}, {"d1"}, {}, {"d4"}},
			rawDimensions:    testRawDimensions,
			want: [][]*cloudwatch.Dimension{
				testRawDimensions,
				{
					{
						Name:  aws.String("d1"),
						Value: aws.String("v1"),
					},
					{
						Name:  aws.String("d2"),
						Value: aws.String("v2"),
					},
				},
				{
					{
						Name:  aws.String("d1"),
						Value: aws.String("v1"),
					},
				},
				{},
			},
		},
		"WithNoRollupConfig": {
			rollupDimensions: [][]string{},
			rawDimensions:    testRawDimensions,
			want:             [][]*cloudwatch.Dimension{testRawDimensions},
		},
		"WithNoRawDimensions": {
			rollupDimensions: [][]string{{"d1", "d2"}, {"d1"}, {}},
			rawDimensions:    []*cloudwatch.Dimension{},
			want:             [][]*cloudwatch.Dimension{{}},
		},
		"WithDuplicate/SameOrder": {
			rollupDimensions: [][]string{{"d1", "d2", "d3"}},
			rawDimensions:    testRawDimensions,
			want:             [][]*cloudwatch.Dimension{testRawDimensions},
		},
		"WithDuplicate/DifferentOrder": {
			rollupDimensions: [][]string{{"d2", "d1", "d3"}},
			rawDimensions:    testRawDimensions,
			want:             [][]*cloudwatch.Dimension{testRawDimensions},
		},
		"WithSameLength/DifferentNames": {
			rollupDimensions: [][]string{{"d1", "d3", "d4"}},
			rawDimensions:    testRawDimensions,
			want:             [][]*cloudwatch.Dimension{testRawDimensions},
		},
		"WithExtraDimensions": {
			rollupDimensions: [][]string{{"d1", "d2", "d3", "d4"}},
			rawDimensions:    testRawDimensions,
			want:             [][]*cloudwatch.Dimension{testRawDimensions},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cw.config.RollupDimensions = testCase.rollupDimensions
			got := cw.ProcessRollup(testCase.rawDimensions)
			assert.EqualValues(t, testCase.want, got, "Unexpected dimension roll up list")
		})
	}
	assert.NoError(t, cw.Shutdown(context.Background()))
}

func TestBuildMetricDatumDropUnsupported(t *testing.T) {
	svc := new(mockCloudWatchClient)
	cw := newCloudWatchClient(svc, time.Second)
	testCases := []float64{
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		distribution.MaxValue * 1.001,
		distribution.MinValue * 1.001,
	}
	for _, testCase := range testCases {
		got := cw.BuildMetricDatum(&aggregationDatum{
			MetricDatum: cloudwatch.MetricDatum{
				MetricName: aws.String("test"),
				Value:      aws.Float64(testCase),
			},
		})
		assert.Empty(t, got)
	}
}

func TestGetUniqueRollupList(t *testing.T) {
	testCases := map[string]struct {
		input [][]string
		want  [][]string
	}{
		"WithEmpty": {
			input: [][]string{},
			want:  [][]string{},
		},
		"WithSimple": {
			input: [][]string{{"d1", "d2", ""}},
			want:  [][]string{{"", "d1", "d2"}},
		},
		"WithDuplicates/NoDimension": {
			input: [][]string{{}, {}},
			want:  [][]string{{}},
		},
		"WithDuplicates/SingleDimension": {
			input: [][]string{{"d1"}, {"d1"}, {"d2"}, {"d1"}},
			want:  [][]string{{"d1"}, {"d2"}},
		},
		"WithDuplicates/DifferentOrder": {
			input: [][]string{{"d2", "d1", "d3"}, {"d3", "d1", "d2"}, {"d3", "d2", "d1"}},
			want:  [][]string{{"d1", "d2", "d3"}},
		},
		"WithDuplicates/WithinSets": {
			input: [][]string{{"d1", "d1", "d2"}, {"d1", "d1"}, {"d2", "d1"}, {"d1"}},
			want:  [][]string{{"d1", "d2"}, {"d1"}},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := GetUniqueRollupList(testCase.input)
			assert.EqualValues(t, testCase.want, got)
		})
	}
}

func TestIsDropping(t *testing.T) {
	svc := new(mockCloudWatchClient)
	cw := newCloudWatchClient(svc, time.Second)

	testCases := map[string]struct {
		dropMetricsConfig    map[string]bool
		expectMetricsDropped map[string]bool
	}{
		"TestIsDroppingWithMultipleCategoryLinux": {
			dropMetricsConfig: map[string]bool{
				"cpu_usage_idle":             true,
				"cpu_time_active":            true,
				"nvidia_smi_utilization_gpu": true,
			},
			expectMetricsDropped: map[string]bool{
				"cpu_usage_idle":  true,
				"cpu_time_active": true,
				"nvidia_smi":      false,
				"cpu_usage_guest": false,
			},
		},
		"TestIsDroppingWithMultipleCategoryWindows": {
			dropMetricsConfig: map[string]bool{
				"cpu usage_idle":             true,
				"cpu time_active":            true,
				"nvidia_smi utilization_gpu": true,
			},
			expectMetricsDropped: map[string]bool{
				"cpu usage_idle":  true,
				"cpu time_active": true,
				"nvidia_smi":      false,
				"cpu usage_guest": false,
			},
		},
		"TestIsDroppingWithMetricDecoration": {
			dropMetricsConfig: map[string]bool{
				"CPU_USAGE_IDLE":             true,
				"cpu_time_active":            true,
				"nvidia_smi_utilization_gpu": true,
			},
			expectMetricsDropped: map[string]bool{
				"cpu_usage_idle":             false,
				"CPU_USAGE_IDLE":             true,
				"nvidia_smi":                 false,
				"nvidia_smi_utilization_gpu": true,
				"cpu":                        false,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cw.config.DropOriginalConfigs = testCase.dropMetricsConfig
			for metricName, expectMetricDropped := range testCase.expectMetricsDropped {
				actualMetricDropped := cw.IsDropping(metricName)
				require.Equal(t, expectMetricDropped, actualMetricDropped)
			}
		})
	}
}

func TestIsFlushable(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	svc.On("PutMetricData", mock.Anything).Return(
		&res,
		nil)
	cw := newCloudWatchClient(svc, time.Second)
	cw.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(10),
		10,
		2*time.Second,
		cw.WriteToCloudWatch)
	assert := assert.New(t)
	perRequestConstSize := overallConstPerRequestSize + len("CWAgent") + namespaceOverheads
	batch := newMetricDatumBatch(defaultMaxDatumsPerCall, perRequestConstSize)
	tags := map[string]string{}
	datum := cloudwatch.MetricDatum{
		MetricName: aws.String("test_metric"),
		Value:      aws.Float64(1),
		Dimensions: BuildDimensions(tags),
		Timestamp:  aws.Time(time.Now()),
	}
	batch.Partition = append(batch.Partition, &datum)
	assert.False(cw.timeToPublish(batch))
	time.Sleep(time.Second + cw.config.ForceFlushInterval)
	assert.True(cw.timeToPublish(batch))
	cw.Shutdown(context.Background())
}

func TestIsFull(t *testing.T) {
	assert := assert.New(t)
	perRequestConstSize := overallConstPerRequestSize + len("CWAgent") + namespaceOverheads
	batch := newMetricDatumBatch(defaultMaxDatumsPerCall, perRequestConstSize)
	tags := map[string]string{}
	datum := cloudwatch.MetricDatum{
		MetricName: aws.String("test_metric"),
		Value:      aws.Float64(1),
		Dimensions: BuildDimensions(tags),
		Timestamp:  aws.Time(time.Now()),
	}
	for i := 0; i < 3; {
		batch.Partition = append(batch.Partition, &datum)
		i++
	}
	assert.False(batch.isFull())
	for i := 0; i < defaultMaxDatumsPerCall-3; {
		batch.Partition = append(batch.Partition, &datum)
		i++
	}
	assert.True(batch.isFull())
}

type mockCloudWatchClient struct {
	cloudwatchiface.CloudWatchAPI
	mock.Mock
}

func (svc *mockCloudWatchClient) PutMetricData(
	input *cloudwatch.PutMetricDataInput,
) (*cloudwatch.PutMetricDataOutput, error) {
	args := svc.Called(input)
	return args.Get(0).(*cloudwatch.PutMetricDataOutput), args.Error(1)
}

func newCloudWatchClient(
	svc cloudwatchiface.CloudWatchAPI,
	forceFlushInterval time.Duration,
) *CloudWatch {
	cloudwatch := &CloudWatch{
		svc: svc,
		config: &Config{
			ForceFlushInterval: forceFlushInterval,
			MaxDatumsPerCall:   defaultMaxDatumsPerCall,
			MaxValuesPerDatum:  defaultMaxValuesPerDatum,
		},
	}
	cloudwatch.startRoutines()
	return cloudwatch
}

func makeMetrics(count int) []telegraf.Metric {
	metrics := make([]telegraf.Metric, 0, count)
	measurement := "Test_namespace"
	fields := map[string]interface{}{
		"usage_user": 100,
	}

	tags := map[string]string{}
	ti := time.Now()
	m := metric.New(measurement, tags, fields, ti)
	for i := 0; i < count; i++ {
		metrics = append(metrics, m.Copy())
	}
	return metrics
}

func TestConsumeMetrics(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	svc.On("PutMetricData", mock.Anything).Return(
		&res,
		nil)
	cloudWatchOutput := newCloudWatchClient(svc, time.Second)
	cloudWatchOutput.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(10), 10, 2*time.Second,
		cloudWatchOutput.WriteToCloudWatch)
	metrics := makeMetrics(1500)
	cloudWatchOutput.Write(metrics)
	time.Sleep(2*time.Second + 2*cloudWatchOutput.config.ForceFlushInterval)
	svc.On("PutMetricData", mock.Anything).Return(&res, nil)
	cw := newCloudWatchClient(svc, time.Second)
	cw.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(10),
		10,
		2*time.Second,
		cw.WriteToCloudWatch)
	// Expect 1500 metrics batched in 2 API calls.
	pmetrics := createTestMetrics(1500, 1, 1, "B/s")
	ctx := context.Background()
	cw.ConsumeMetrics(ctx, pmetrics)
	time.Sleep(2*time.Second + 2*cw.config.ForceFlushInterval)
	assert.True(t, svc.AssertNumberOfCalls(t, "PutMetricData", 2))
	cw.Shutdown(ctx)
}

func TestWriteError(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	serverInternalErr := awserr.New(cloudwatch.ErrCodeLimitExceededFault, "", nil)
	svc.On("PutMetricData", mock.Anything).Return(
		&res,
		serverInternalErr)
	cw := newCloudWatchClient(svc, time.Second)
	cw.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(10),
		10,
		2*time.Second,
		cw.WriteToCloudWatch)
	metrics := createTestMetrics(20, 1, 10, "")
	ctx := context.Background()
	cw.ConsumeMetrics(ctx, metrics)

	// Sum time for all retries.
	var sum int
	for i := 0; i < defaultRetryCount; i++ {
		sum += 1 << i
	}
	time.Sleep(backoffRetryBase * time.Duration(sum))
	assert.True(t, svc.AssertNumberOfCalls(t, "PutMetricData", 5))
	cw.Shutdown(ctx)
}

// TestPublish verifies metric batches do not get pushed immediately when
// batch-buffer is full.
func TestPublish(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	svc.On("PutMetricData", mock.Anything).Return(
		&res,
		nil)
	interval := 60 * time.Second
	// The buffer holds 50 batches of 1,000 metrics. So choose 5x.
	numMetrics := 5 * datumBatchChanBufferSize * defaultMaxDatumsPerCall
	expectedCalls := numMetrics / defaultMaxDatumsPerCall
	log.Printf("I! interval %v, numMetrics %v, expectedCalls %v",
		interval, numMetrics, expectedCalls)
	cw := newCloudWatchClient(svc, interval)
	cw.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(metricChanBufferSize),
		maxConcurrentPublisher,
		2*time.Second,
		cw.WriteToCloudWatch)
	metrics := createTestMetrics(numMetrics, 1, 1, "")
	ctx := context.Background()
	// Use goroutine since it could block if len(metrics) >metricChanBufferSize.
	go cw.ConsumeMetrics(ctx, metrics)
	// Expect some, but not all API calls after half the original interval.
	time.Sleep(interval/2 + 2*time.Second)
	assert.Less(t, 0, len(svc.Calls))
	assert.Less(t, len(svc.Calls), expectedCalls)
	// Expect all API calls after 1.5x the interval.
	// 10K metrics in batches of 20...
	time.Sleep(interval)
	assert.Equal(t, expectedCalls, len(svc.Calls))
	cw.Shutdown(ctx)
}

func TestMiddleware(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	newType, _ := component.NewType("test")
	id := component.NewID(newType)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cw := &CloudWatch{
		config: &Config{
			Region:             "test-region",
			Namespace:          "test-namespace",
			ForceFlushInterval: time.Second,
			EndpointOverride:   server.URL,
			MiddlewareID:       &id,
		},
		logger: zap.NewNop(),
	}
	ctx := context.Background()
	handler := new(awsmiddleware.MockHandler)
	handler.On("ID").Return("test")
	handler.On("Position").Return(awsmiddleware.After)
	handler.On("HandleRequest", mock.Anything, mock.Anything)
	handler.On("HandleResponse", mock.Anything, mock.Anything)
	middleware := new(awsmiddleware.MockMiddlewareExtension)
	middleware.On("Handlers").Return([]awsmiddleware.RequestHandler{handler}, []awsmiddleware.ResponseHandler{handler})
	extensions := map[component.ID]component.Component{id: middleware}
	host := new(awsmiddleware.MockExtensionsHost)
	host.On("GetExtensions").Return(extensions)
	assert.NoError(t, cw.Start(ctx, host))
	// Expect 1500 metrics batched in 2 API calls.
	pmetrics := createTestMetrics(1500, 1, 1, "B/s")
	assert.NoError(t, cw.ConsumeMetrics(ctx, pmetrics))
	time.Sleep(2*time.Second + 2*cw.config.ForceFlushInterval)
	handler.AssertCalled(t, "HandleRequest", mock.Anything, mock.Anything)
	handler.AssertCalled(t, "HandleResponse", mock.Anything, mock.Anything)
	require.NoError(t, cw.Shutdown(ctx))
}

func TestBackoffRetries(t *testing.T) {
	c := &CloudWatch{}
	sleeps := []time.Duration{
		time.Millisecond * 200,
		time.Millisecond * 400,
		time.Millisecond * 800,
		time.Millisecond * 1600,
		time.Millisecond * 3200,
		time.Millisecond * 6400}
	assert := assert.New(t)
	leniency := 200 * time.Millisecond
	for i := 0; i <= defaultRetryCount; i++ {
		start := time.Now()
		c.backoffSleep()
		// Expect time since start is between sleeps[i]/2 and sleeps[i].
		// Except that github automation fails on this for MacOs, so allow leniency.
		assert.Less(sleeps[i]/2, time.Since(start))
		assert.Greater(sleeps[i]+leniency, time.Since(start))
	}
	start := time.Now()
	c.backoffSleep()
	assert.Less(30*time.Second, time.Since(start))
	assert.Greater(60*time.Second, time.Since(start))
	// reset
	c.retries = 0
	start = time.Now()
	c.backoffSleep()
	assert.Greater(200*time.Millisecond+leniency, time.Since(start))
}

// Fill up the channel and verify it is full.
// Take 1 item out of the channel and verify it is no longer full.
func TestCloudWatch_metricDatumBatchFull(t *testing.T) {
	c := &CloudWatch{
		datumBatchChan: make(chan []*cloudwatch.MetricDatum, datumBatchChanBufferSize),
	}
	assert.False(t, c.metricDatumBatchFull())
	for i := 0; i < datumBatchChanBufferSize; i++ {
		c.datumBatchChan <- []*cloudwatch.MetricDatum{}
	}
	assert.True(t, c.metricDatumBatchFull())
	<-c.datumBatchChan
	assert.False(t, c.metricDatumBatchFull())
}
