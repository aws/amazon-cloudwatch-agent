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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/useragent"
	"github.com/aws/amazon-cloudwatch-agent/internal/publisher"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

type mockMiddleware struct {
	mock.Mock
}

var _ smithymiddleware.BuildMiddleware = (*mockMiddleware)(nil)

func (m *mockMiddleware) ID() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockMiddleware) HandleBuild(ctx context.Context, in smithymiddleware.BuildInput, next smithymiddleware.BuildHandler) (smithymiddleware.BuildOutput, smithymiddleware.Metadata, error) {
	args := m.Called(ctx, in, next)
	return args.Get(0).(smithymiddleware.BuildOutput), args.Get(1).(smithymiddleware.Metadata), args.Error(2)
}

// Test that each tag becomes one dimension.
// Test that no more than 30 dimensions will get returned.
// Test that if "host" dimension exists, it is always included.
func TestBuildDimensions(t *testing.T) {
	dims := BuildDimensions(nil)
	assert.Equal(t, 0, len(dims))
	// empty
	dims = BuildDimensions(make(map[string]string))
	assert.Equal(t, 0, len(dims))
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

		assert.Equal(t, expectedLen, valCount)
		if i%2 == 0 {
			assert.Equal(t, 1, hostCount)
			assert.Equal(t, expectedLen-1, keyCount)
		} else {
			assert.Equal(t, 0, hostCount)
			assert.Equal(t, expectedLen, keyCount)
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

	testRawDimensions := []types.Dimension{
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
		rawDimensions    []types.Dimension
		want             [][]types.Dimension
	}{
		"WithSimpleRollup": {
			rollupDimensions: [][]string{{"d1", "d2"}, {"d1"}, {}, {"d4"}},
			rawDimensions:    testRawDimensions,
			want: [][]types.Dimension{
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
			want:             [][]types.Dimension{testRawDimensions},
		},
		"WithNoRawDimensions": {
			rollupDimensions: [][]string{{"d1", "d2"}, {"d1"}, {}},
			rawDimensions:    []types.Dimension{},
			want:             [][]types.Dimension{{}},
		},
		"WithDuplicate/SameOrder": {
			rollupDimensions: [][]string{{"d1", "d2", "d3"}},
			rawDimensions:    testRawDimensions,
			want:             [][]types.Dimension{testRawDimensions},
		},
		"WithDuplicate/DifferentOrder": {
			rollupDimensions: [][]string{{"d2", "d1", "d3"}},
			rawDimensions:    testRawDimensions,
			want:             [][]types.Dimension{testRawDimensions},
		},
		"WithSameLength/DifferentNames": {
			rollupDimensions: [][]string{{"d1", "d3", "d4"}},
			rawDimensions:    testRawDimensions,
			want:             [][]types.Dimension{testRawDimensions},
		},
		"WithExtraDimensions": {
			rollupDimensions: [][]string{{"d1", "d2", "d3", "d4"}},
			rawDimensions:    testRawDimensions,
			want:             [][]types.Dimension{testRawDimensions},
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

	_, datums := cw.BuildMetricDatum(&aggregationDatum{
		MetricDatum: types.MetricDatum{
			MetricName: aws.String("test_nil_value"),
			Value:      nil,
		},
	})
	assert.Empty(t, datums)

	testCases := []float64{
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		distribution.MaxValue * 1.001,
		distribution.MinValue * 1.001,
	}
	for _, testCase := range testCases {
		_, datums := cw.BuildMetricDatum(&aggregationDatum{
			MetricDatum: types.MetricDatum{
				MetricName: aws.String("test"),
				Value:      aws.Float64(testCase),
			},
		})
		assert.Empty(t, datums)
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
	svc.On("PutMetricData", mock.Anything, mock.Anything, mock.Anything).Return(
		&res,
		nil)
	cw := newCloudWatchClient(svc, time.Second)
	cw.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(10),
		10,
		2*time.Second,
		cw.WriteToCloudWatch)
	perRequestConstSize := overallConstPerRequestSize + len("CWAgent") + namespaceOverheads
	batch := newMetricDatumBatch(defaultMaxDatumsPerCall, perRequestConstSize)
	tags := map[string]string{}
	datum := types.MetricDatum{
		MetricName: aws.String("test_metric"),
		Value:      aws.Float64(1),
		Dimensions: BuildDimensions(tags),
		Timestamp:  aws.Time(time.Now()),
	}
	batch.Partition = map[string][]types.MetricDatum{
		"TestEntity": append([]types.MetricDatum{}, datum),
	}
	assert.False(t, cw.timeToPublish(batch))
	time.Sleep(time.Second + cw.config.ForceFlushInterval)
	assert.True(t, cw.timeToPublish(batch))
	assert.NoError(t, cw.Shutdown(context.Background()))
}

func TestIsFull(t *testing.T) {
	perRequestConstSize := overallConstPerRequestSize + len("CWAgent") + namespaceOverheads
	batch := newMetricDatumBatch(defaultMaxDatumsPerCall, perRequestConstSize)
	tags := map[string]string{}
	datum := types.MetricDatum{
		MetricName: aws.String("test_metric"),
		Value:      aws.Float64(1),
		Dimensions: BuildDimensions(tags),
		Timestamp:  aws.Time(time.Now()),
	}
	batch.Partition = map[string][]types.MetricDatum{
		"TestEntity": {},
	}
	partition := batch.Partition["TestEntity"]
	for i := 0; i < 3; {
		batch.Partition["TestEntity"] = append(partition, datum)
		batch.Count++
		i++
	}
	assert.False(t, batch.isFull())
	for i := 0; i < defaultMaxDatumsPerCall-3; {
		batch.Partition["TestEntity"] = append(partition, datum)
		batch.Count++
		i++
	}
	assert.True(t, batch.isFull())
}

type mockCloudWatchClient struct {
	mock.Mock
}

var _ PutMetricDataAPI = (*mockCloudWatchClient)(nil)

func (m *mockCloudWatchClient) PutMetricData(
	ctx context.Context,
	input *cloudwatch.PutMetricDataInput,
	optFns ...func(*cloudwatch.Options),
) (*cloudwatch.PutMetricDataOutput, error) {
	args := m.Called(ctx, input, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*cloudwatch.PutMetricDataOutput), args.Error(1)
}

func newCloudWatchClient(
	client PutMetricDataAPI,
	forceFlushInterval time.Duration,
) *CloudWatch {
	cw := &CloudWatch{
		client: client,
		config: &Config{
			ForceFlushInterval: forceFlushInterval,
			MaxDatumsPerCall:   defaultMaxDatumsPerCall,
			MaxValuesPerDatum:  defaultMaxValuesPerDatum,
		},
	}
	cw.startRoutines()
	return cw
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
	svc.On("PutMetricData", mock.Anything, mock.Anything, mock.Anything).Return(
		&res,
		nil)
	cloudWatchOutput := newCloudWatchClient(svc, time.Second)
	cloudWatchOutput.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(10), 10, 2*time.Second,
		cloudWatchOutput.WriteToCloudWatch)
	metrics := makeMetrics(1500)
	assert.NoError(t, cloudWatchOutput.Write(metrics))
	time.Sleep(2*time.Second + 2*cloudWatchOutput.config.ForceFlushInterval)
	svc.On("PutMetricData", mock.Anything, mock.Anything, mock.Anything).Return(&res, nil)
	cw := newCloudWatchClient(svc, time.Second)
	cw.publisher, _ = publisher.NewPublisher(
		publisher.NewNonBlockingFifoQueue(10),
		10,
		2*time.Second,
		cw.WriteToCloudWatch)
	// Expect 1500 metrics batched in 2 API calls.
	pmetrics := createTestMetrics(1500, 1, 1, "B/s")
	ctx := context.Background()
	assert.NoError(t, cw.ConsumeMetrics(ctx, pmetrics))
	time.Sleep(2*time.Second + 2*cw.config.ForceFlushInterval)
	assert.True(t, svc.AssertNumberOfCalls(t, "PutMetricData", 2))
	assert.NoError(t, cw.Shutdown(ctx))
}

func TestWriteError(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	serverInternalErr := &types.LimitExceededFault{
		Message: aws.String("Limit exceeded"),
	}
	svc.On("PutMetricData", mock.Anything, mock.Anything, mock.Anything).Return(
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
	assert.NoError(t, cw.ConsumeMetrics(ctx, metrics))

	// Sum time for all retries.
	var sum int
	for i := 0; i < defaultRetryCount; i++ {
		sum += 1 << i
	}
	time.Sleep(backoffRetryBase * time.Duration(sum))
	assert.True(t, svc.AssertNumberOfCalls(t, "PutMetricData", 5))
	assert.NoError(t, cw.Shutdown(ctx))
}

// TestPublish verifies metric batches do not get pushed immediately when
// batch-buffer is full.
func TestPublish(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	svc.On("PutMetricData", mock.Anything, mock.Anything, mock.Anything).Return(
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
	go func() {
		assert.NoError(t, cw.ConsumeMetrics(ctx, metrics))
	}()
	// Expect some, but not all API calls after half the original interval.
	time.Sleep(interval/2 + 2*time.Second)
	assert.Less(t, 0, len(svc.Calls))
	assert.Less(t, len(svc.Calls), expectedCalls)
	// Expect all API calls after 1.5x the interval.
	// 10K metrics in batches of 20...
	time.Sleep(interval)
	assert.Equal(t, expectedCalls, len(svc.Calls))
	assert.Equal(t, 0, metrics.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.NoError(t, cw.Shutdown(ctx))
}

func TestMiddleware(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	newType, _ := component.NewType("test")
	id := component.NewID(newType)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("smithy-protocol", "rpc-v2-cbor")
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
	leniency := 200 * time.Millisecond
	for i := 0; i <= defaultRetryCount; i++ {
		start := time.Now()
		c.backoffSleep()
		// Expect time since start is between sleeps[i]/2 and sleeps[i].
		// Except that github automation fails on this for MacOs, so allow leniency.
		assert.Less(t, sleeps[i]/2, time.Since(start))
		assert.Greater(t, sleeps[i]+leniency, time.Since(start))
	}
	start := time.Now()
	c.backoffSleep()
	assert.Less(t, 30*time.Second, time.Since(start))
	assert.Greater(t, 60*time.Second, time.Since(start))
	// reset
	c.retries = 0
	start = time.Now()
	c.backoffSleep()
	assert.Greater(t, 200*time.Millisecond+leniency, time.Since(start))
}

// Fill up the channel and verify it is full.
// Take 1 item out of the channel and verify it is no longer full.
func TestCloudWatch_metricDatumBatchFull(t *testing.T) {
	c := &CloudWatch{
		datumBatchChan: make(chan map[string][]types.MetricDatum, datumBatchChanBufferSize),
	}
	assert.False(t, c.metricDatumBatchFull())
	for i := 0; i < datumBatchChanBufferSize; i++ {
		c.datumBatchChan <- map[string][]types.MetricDatum{}
	}
	assert.True(t, c.metricDatumBatchFull())
	<-c.datumBatchChan
	assert.False(t, c.metricDatumBatchFull())
}

func TestCreateEntityMetricData(t *testing.T) {
	svc := new(mockCloudWatchClient)
	cw := newCloudWatchClient(svc, time.Second)
	metrics := createTestMetrics(1, 1, 1, "s")
	assert.Equal(t, 7, metrics.ResourceMetrics().At(0).Resource().Attributes().Len())
	aggregations := convertOtelMetrics(metrics)
	assert.Equal(t, 0, metrics.ResourceMetrics().At(0).Resource().Attributes().Len())
	entity, metricDatum := cw.BuildMetricDatum(aggregations[0])

	entityToMetrics := map[string][]types.MetricDatum{
		entityToString(entity): metricDatum,
	}
	wantedEntityMetricData := []types.EntityMetricData{
		{
			Entity:     &entity,
			MetricData: metricDatum,
		},
	}
	assert.Equal(t, wantedEntityMetricData, createEntityMetricData(entityToMetrics))
}

func TestWriteToCloudWatchEntity(t *testing.T) {
	timestampNow := aws.Time(time.Now())
	expectedPMDInput := &cloudwatch.PutMetricDataInput{
		Namespace:              aws.String(""),
		StrictEntityValidation: aws.Bool(false),
		EntityMetricData: []types.EntityMetricData{
			{
				Entity: &types.Entity{
					Attributes: map[string]string{},
					KeyAttributes: map[string]string{
						"Environment": "Environment",
						"Service":     "Service",
					},
				},
				MetricData: []types.MetricDatum{
					{
						MetricName: aws.String("TestMetricWithEntity"),
						Value:      aws.Float64(1),
						Timestamp:  timestampNow,
						Dimensions: []types.Dimension{
							{Name: aws.String("Class"), Value: aws.String("class")},
							{Name: aws.String("Object"), Value: aws.String("object")},
						},
					},
				},
			},
		},
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String("TestMetricNoEntity"),
				Value:      aws.Float64(1),
				Timestamp:  timestampNow,
				Dimensions: []types.Dimension{
					{Name: aws.String("Class"), Value: aws.String("class")},
					{Name: aws.String("Object"), Value: aws.String("object")},
				},
			},
		},
	}

	var input *cloudwatch.PutMetricDataInput
	svc := new(mockCloudWatchClient)
	svc.On("PutMetricData", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		input = args.Get(1).(*cloudwatch.PutMetricDataInput)
	}).Return(&cloudwatch.PutMetricDataOutput{}, nil)

	cw := newCloudWatchClient(svc, time.Second)
	cw.WriteToCloudWatch(map[string][]types.MetricDatum{
		"": {
			{
				MetricName: aws.String("TestMetricNoEntity"),
				Value:      aws.Float64(1),
				Timestamp:  timestampNow,
				Dimensions: []types.Dimension{
					{Name: aws.String("Class"), Value: aws.String("class")},
					{Name: aws.String("Object"), Value: aws.String("object")},
				},
			},
		},
		"|Environment:Environment;Service:Service": {
			{
				MetricName: aws.String("TestMetricWithEntity"),
				Value:      aws.Float64(1),
				Timestamp:  timestampNow,
				Dimensions: []types.Dimension{
					{Name: aws.String("Class"), Value: aws.String("class")},
					{Name: aws.String("Object"), Value: aws.String("object")},
				},
			},
		},
	})

	assert.Equal(t, expectedPMDInput, input)
}

func TestUserAgentFeatureFlags(t *testing.T) {
	testCases := []struct {
		name               string
		metricNames        []string
		expectedFeatureStr string
	}{
		{
			name:               "NoFeatures",
			metricNames:        []string{"other_metric"},
			expectedFeatureStr: "",
		},
		{
			name:               "EBSOnly",
			metricNames:        []string{"diskio_ebs_total_read_ops"},
			expectedFeatureStr: " feature:(nvme_ebs)",
		},
		{
			name:               "InstanceStoreOnly",
			metricNames:        []string{"diskio_instance_store_total_read_ops"},
			expectedFeatureStr: " feature:(nvme_is)",
		},
		{
			name:               "BothFeatures",
			metricNames:        []string{"diskio_ebs_total_read_ops", "diskio_instance_store_total_read_ops"},
			expectedFeatureStr: " feature:(nvme_ebs nvme_is)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			useragent.Get().Reset()

			cfg := aws.Config{
				Region: "us-west-2",
			}
			handler := useragent.NewHandler(true)
			configurer := awsmiddleware.NewConfigurer([]awsmiddleware.RequestHandler{handler}, nil)
			require.NoError(t, configurer.Configure(awsmiddleware.SDKv2(&cfg)))
			client := cloudwatch.NewFromConfig(cfg, func(o *cloudwatch.Options) {
				o.BaseEndpoint = aws.String("http://localhost:12345")
			})
			cw := &CloudWatch{
				client: client,
				config: &Config{
					ForceFlushInterval: time.Second,
				},
				logger: zap.NewNop(),
			}

			// Process metrics to trigger detection
			for _, name := range tc.metricNames {
				cw.handleMetricName(name)
			}

			var got string
			mm := new(mockMiddleware)
			mm.On("ID").Return("captureUserAgent")
			mm.On("HandleBuild", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				if in, ok := args.Get(1).(smithymiddleware.BuildInput); ok {
					if req, ok := in.Request.(*smithyhttp.Request); ok {
						got = req.Header.Get("User-Agent")
					}
				}
			}).Return(smithymiddleware.BuildOutput{
				Result: &cloudwatch.PutMetricDataOutput{},
			}, smithymiddleware.Metadata{}, nil)
			_, err := client.PutMetricData(t.Context(), &cloudwatch.PutMetricDataInput{
				Namespace: aws.String("test"),
				MetricData: []types.MetricDatum{
					{
						MetricName: aws.String("test"),
						Value:      aws.Float64(1),
					},
				},
			}, func(o *cloudwatch.Options) {
				o.APIOptions = append(o.APIOptions, func(s *smithymiddleware.Stack) error {
					return s.Build.Add(mm, smithymiddleware.After)
				})
			})
			require.NoError(t, err)
			assert.Contains(t, got, tc.expectedFeatureStr)
		})
	}
}
