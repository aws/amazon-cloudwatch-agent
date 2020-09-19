// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/internal/publisher"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/testutil"
	"github.com/influxdata/toml"
	"github.com/influxdata/toml/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test that each tag becomes one dimension
func TestBuildDimensions(t *testing.T) {
	const MaxDimensions = 10

	assert := assert.New(t)

	testPoint := testutil.TestMetric(1)
	dimensions := BuildDimensions(testPoint.Tags())

	tagKeys := make([]string, len(testPoint.Tags()))
	i := 0
	for k, _ := range testPoint.Tags() {
		tagKeys[i] = k
		i += 1
	}

	sort.Strings(tagKeys)

	if len(testPoint.Tags()) >= MaxDimensions {
		assert.Equal(MaxDimensions, len(dimensions), "Number of dimensions should be less than MaxDimensions")
	} else {
		assert.Equal(len(testPoint.Tags()), len(dimensions), "Number of dimensions should be equal to number of tags")
	}

	for i, key := range tagKeys {
		if i >= 10 {
			break
		}
		assert.Equal(key, *dimensions[i].Name, "Key should be equal")
		assert.Equal(testPoint.Tags()[key], *dimensions[i].Value, "Value should be equal")
	}
}

// Test that metrics with valid values have a MetricDatum created where as non valid do not.
// Skips "time.Time" type as something is converting the value to string.
func TestBuildMetricDatums(t *testing.T) {
	assert := assert.New(t)

	c := &CloudWatch{MaxValuesPerDatum: 3}

	highResolutionMetric := testutil.TestMetric(0)
	highResolutionMetric.RemoveTag("tag1")
	highResolutionMetric.AddTag(highResolutionTagKey, "true")

	hdatums := c.BuildMetricDatum(highResolutionMetric)
	assert.Equal(1, len(hdatums), "Should be able to create one high resolution Datum")
	assert.Equal(0, len(hdatums[0].Dimensions), "The high resolution tags shouldn't be build into metric")

	distribution.NewDistribution = regular.NewRegularDistribution

	validDistribution := distribution.NewDistribution()
	validDistribution.AddEntry(1, 1)
	validMetrics := []telegraf.Metric{
		testutil.TestMetric(1),
		testutil.TestMetric(int32(1)),
		testutil.TestMetric(int64(1)),
		testutil.TestMetric(float64(1)),
		testutil.TestMetric(true),
		testutil.TestMetric(validDistribution),
	}

	for _, point := range validMetrics {
		datums := c.BuildMetricDatum(point)
		assert.Equal(1, len(datums), "Valid type should create a Datum")
	}

	invalidDistribution := distribution.NewDistribution()
	invalidDistribution.AddEntry(-1, 1)
	invalidMetrics := []telegraf.Metric{
		testutil.TestMetric("Foo"),
		testutil.TestMetric(invalidDistribution),
	}

	for _, point := range invalidMetrics {
		datums := c.BuildMetricDatum(point)
		assert.Equal(0, len(datums), "Invalid type/value should not create a Datum")
	}
}

func TestProcessRollup(t *testing.T) {
	svc := new(mockCloudWatchClient)
	cloudWatchOutput := newCloudWatchClient(svc)
	cloudWatchOutput.RollupDimensions = [][]string{{"d1", "d2"}, {"d1"}, {}, {"d4"}}

	rawDimension := []*cloudwatch.Dimension{
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

	actualDimensionList := cloudWatchOutput.ProcessRollup(rawDimension)
	expectedDimensionList := [][]*cloudwatch.Dimension{
		{
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
		},
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
	}
	assert.EqualValues(t, expectedDimensionList, actualDimensionList, "Unexpected dimension roll up list")

	cloudWatchOutput.RollupDimensions = [][]string{}
	rawDimension = []*cloudwatch.Dimension{
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

	actualDimensionList = cloudWatchOutput.ProcessRollup(rawDimension)
	expectedDimensionList = [][]*cloudwatch.Dimension{
		{
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
		},
	}
	assert.EqualValues(t, expectedDimensionList, actualDimensionList, "Unexpected dimension roll up list without rollup setting")

	cloudWatchOutput.RollupDimensions = [][]string{{"d1", "d2"}, {"d1"}, {}}
	rawDimension = []*cloudwatch.Dimension{}

	actualDimensionList = cloudWatchOutput.ProcessRollup(rawDimension)
	expectedDimensionList = [][]*cloudwatch.Dimension{
		{},
	}
	assert.EqualValues(t, expectedDimensionList, actualDimensionList, "Unexpected dimension roll up list with no raw dimensions")

	cloudWatchOutput.RollupDimensions = [][]string{{"d1", "d2", "d3"}}
	rawDimension = []*cloudwatch.Dimension{
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

	actualDimensionList = cloudWatchOutput.ProcessRollup(rawDimension)
	expectedDimensionList = [][]*cloudwatch.Dimension{
		{
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
		},
	}
	assert.EqualValues(t, expectedDimensionList, actualDimensionList, "Unexpected dimension roll up list with duplicate roll up")
}

func TestGetUniqueRollupList(t *testing.T) {
	inputLists := [][]string{{"d1"}, {"d1"}, {"d2"}, {"d1"}}
	actualLists := GetUniqueRollupList(inputLists)
	expectedLists := [][]string{{"d1"}, {"d2"}}
	assert.EqualValues(t, expectedLists, actualLists, "Duplicate list showed up")

	inputLists = [][]string{{"d1", "d2", ""}}
	actualLists = GetUniqueRollupList(inputLists)
	expectedLists = [][]string{{"d1", "d2", ""}}
	assert.EqualValues(t, expectedLists, actualLists, "Unique list should be same with input list")

	inputLists = [][]string{{}, {}}
	actualLists = GetUniqueRollupList(inputLists)
	expectedLists = [][]string{{}}
	assert.EqualValues(t, expectedLists, actualLists, "Unique list failed on empty list")

	inputLists = [][]string{}
	actualLists = GetUniqueRollupList(inputLists)
	expectedLists = [][]string{}
	assert.EqualValues(t, expectedLists, actualLists, "Unique list result should be empty")
}

func TestIsFlushable(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	svc.On("PutMetricData", mock.Anything).Return(
		&res,
		nil)
	cloudWatchOutput := newCloudWatchClient(svc)

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
	assert.False(cloudWatchOutput.timeToPublish(batch))
	time.Sleep(time.Second + cloudWatchOutput.ForceFlushInterval.Duration)
	assert.True(cloudWatchOutput.timeToPublish(batch))
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

func (svc *mockCloudWatchClient) PutMetricData(input *cloudwatch.PutMetricDataInput) (*cloudwatch.PutMetricDataOutput, error) {
	args := svc.Called(input)
	return args.Get(0).(*cloudwatch.PutMetricDataOutput), args.Error(1)
}

func newCloudWatchClient(svc cloudwatchiface.CloudWatchAPI) *CloudWatch {
	cloudwatch := &CloudWatch{
		svc:                svc,
		ForceFlushInterval: internal.Duration{Duration: 1 * time.Second},
	}
	cloudwatch.startRoutines()
	return cloudwatch
}

func TestWrite(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	svc.On("PutMetricData", mock.Anything).Return(
		&res,
		nil)
	cloudWatchOutput := newCloudWatchClient(svc)
	cloudWatchOutput.publisher, _ = publisher.NewPublisher(publisher.NewNonBlockingFifoQueue(10), 10, 2*time.Second, cloudWatchOutput.WriteToCloudWatch)
	metrics := make([]telegraf.Metric, 0, 3)
	measurement := "Test_namespace"
	fields := map[string]interface{}{
		"usage_user":       100,
		"usage_system":     100,
		"usage_idle":       100,
		"usage_nice":       100,
		"usage_iowait":     100,
		"usage_irq":        100,
		"usage_softirq":    100,
		"usage_steal":      100,
		"usage_guest":      100,
		"usage_guest_nice": 100,
	}

	tags := map[string]string{"dimension_name1": "dimension_value2"}
	ti := time.Now()
	m, _ := metric.New(measurement, tags, fields, ti)
	for i := 0; i < 3; i++ {
		metrics = append(metrics, m.Copy())
	}

	cloudWatchOutput.Write(metrics)
	time.Sleep(time.Second + 2*cloudWatchOutput.ForceFlushInterval.Duration)
	cloudWatchOutput.Close()
	assert.True(t, svc.AssertNumberOfCalls(t, "PutMetricData", 2))
}

func TestWriteError(t *testing.T) {
	svc := new(mockCloudWatchClient)
	res := cloudwatch.PutMetricDataOutput{}
	serverInternalErr := awserr.New(cloudwatch.ErrCodeLimitExceededFault, "", nil)
	svc.On("PutMetricData", mock.Anything).Return(
		&res,
		serverInternalErr)
	cloudWatchOutput := newCloudWatchClient(svc)
	cloudWatchOutput.publisher, _ = publisher.NewPublisher(publisher.NewNonBlockingFifoQueue(10), 10, 2*time.Second, cloudWatchOutput.WriteToCloudWatch)
	metrics := make([]telegraf.Metric, 0, 2)
	measurement := "Test_namespace"
	fields := map[string]interface{}{
		"usage_user":       100,
		"usage_system":     100,
		"usage_idle":       100,
		"usage_nice":       100,
		"usage_iowait":     100,
		"usage_irq":        100,
		"usage_softirq":    100,
		"usage_steal":      100,
		"usage_guest":      100,
		"usage_guest_nice": 100,
	}

	tags := map[string]string{"dimension_name1": "dimension_value2"}
	ti := time.Now()
	m, _ := metric.New(measurement, tags, fields, ti)
	for i := 0; i < 2; {
		metrics = append(metrics, m.Copy())
		i++
	}
	cloudWatchOutput.Write(metrics)

	var sum float64
	for i := 0; i < defaultRetryCount; i++ {
		sum = sum + math.Pow(2, float64(i))
	}
	time.Sleep(time.Duration(backoffRetryBase*int64(sum)) * time.Millisecond)

	assert.True(t, svc.AssertNumberOfCalls(t, "PutMetricData", 5))

}

func TestMetricConfigsRead(t *testing.T) {
	contents := `[[outputs.cloudwatch.metric_decoration]]
                     category = "cpu"
                     name     = "cpu"
                     rename   = "CPU"
                     unit     = "Percent"
                 [[outputs.cloudwatch.metric_decoration]]
                     category = "mem"
                     name     = "mem"
                     unit     = "Megabytes"
                 [[outputs.cloudwatch.metric_decoration]]
                     category = "disk"
                     name     = "disk"
                     rename   = "DISK"
                 `

	c, err := buildCloudWatchFromToml(contents)

	assert.True(t, err == nil)

	expected := make([]MetricDecorationConfig, 0)

	mdc := MetricDecorationConfig{
		Category: "cpu",
		Metric:   "cpu",
		Rename:   "CPU",
		Unit:     "Percent",
	}
	expected = append(expected, mdc)

	mdc = MetricDecorationConfig{
		Category: "mem",
		Metric:   "mem",
		Unit:     "Megabytes",
	}
	expected = append(expected, mdc)

	mdc = MetricDecorationConfig{
		Category: "disk",
		Metric:   "disk",
		Rename:   "DISK",
	}
	expected = append(expected, mdc)

	assert.Equal(t, expected, c.MetricConfigs)
}

func TestMissMetricConfig(t *testing.T) {
	contents := `[outputs.cloudwatch]
                     access_key = "metric_access_key"
                     force_flush_interval = "30s"
                `
	c, err := buildCloudWatchFromToml(contents)

	assert.True(t, err == nil)

	assert.True(t, c.MetricConfigs == nil)
}

func buildCloudWatchFromToml(contents string) (*CloudWatch, error) {
	c := &CloudWatch{}

	tbl, err := toml.Parse([]byte(contents))

	if err != nil {
		return c, err
	}

	if outputsVal, ok := tbl.Fields["outputs"]; ok {
		outputsTbl, ok := outputsVal.(*ast.Table)
		if !ok {
			return c, fmt.Errorf("unexpected outputs field")
		}
		cloudWatchVal, ok := outputsTbl.Fields["cloudwatch"]
		if !ok {
			return c, fmt.Errorf("miss cloudwatch field")
		}
		cloudWatchTbl, ok := cloudWatchVal.(*ast.Table)
		if !ok {
			return c, fmt.Errorf("unexpected cloudwatch field")
		}

		if err := toml.UnmarshalTable(cloudWatchTbl, c); err != nil {
			return c, err
		}
	}

	return c, nil
}

func TestBackoffRetries(t *testing.T) {
	c := &CloudWatch{}
	sleeps := []time.Duration{time.Millisecond * 200, time.Millisecond * 400, time.Millisecond * 800,
		time.Millisecond * 1600, time.Millisecond * 3200, time.Millisecond * 6400}
	for i := 0; i <= defaultRetryCount; i++ {
		now := time.Now()
		c.backoffSleep()
		assert.True(t, math.Abs((time.Now().Sub(now)-sleeps[i]).Seconds()) < 1)
	}
	now := time.Now()
	c.backoffSleep()
	assert.True(t, math.Abs((time.Now().Sub(now)-time.Minute).Seconds()) < 1)

	c.retries = 0
	now = time.Now()
	c.backoffSleep()
	assert.True(t, math.Abs((time.Now().Sub(now)-sleeps[0]).Seconds()) < 1)
}

func TestCloudWatch_metricDatumBatchFull(t *testing.T) {
	c := &CloudWatch{
		datumBatchChan:     make(chan []*cloudwatch.MetricDatum, datumBatchChanBufferSize),
		datumBatchFullChan: make(chan bool, 1),
	}

	select {
	case <-c.metricDatumBatchFull():
		assert.Fail(t, "program should not enter metricDatumBatchFull")
	default:
	}

	for i := 0; i < datumBatchChanBufferSize; i++ {
		c.datumBatchChan <- []*cloudwatch.MetricDatum{}
	}

	select {
	case <-c.metricDatumBatchFull():
	default:
		assert.Fail(t, "program should enter metricDatumBatchFull")
	}

	<-c.datumBatchChan

	select {
	case <-c.metricDatumBatchFull():
		assert.Fail(t, "program should not enter metricDatumBatchFull")
	default:
	}

}

func TestBuildMetricDatums_SkipEmptyTags(t *testing.T) {
	c := &CloudWatch{
		datumBatchChan:     make(chan []*cloudwatch.MetricDatum, 0),
		datumBatchFullChan: make(chan bool, 1),
	}
	input := testutil.MustMetric(
		"cpu",
		map[string]string{
			"host": "example.org",
			"foo":  "",
		},
		map[string]interface{}{
			"value": int64(42),
		},
		time.Unix(0, 0),
	)

	datums := c.BuildMetricDatum(input)
	require.Len(t, datums[0].Dimensions, 1)
}
