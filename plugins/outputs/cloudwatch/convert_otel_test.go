// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatch"
)

const (
	keyPrefix      = "key"
	valPrefix      = "val"
	namePrefix     = "metric_name_"
	metricValue    = 24.0
	histogramMin   = 3.0
	histogramMax   = 37.0
	histogramSum   = 4095.0
	histogramCount = 987
)

func addDimensions(attributes pcommon.Map, count int) {
	for i := 0; i < count; i++ {
		key := keyPrefix + strconv.Itoa(i)
		val := valPrefix + strconv.Itoa(i)
		attributes.PutStr(key, val)
	}
}

// createTestMetrics will create the numMetrics metrics.
// Each metric will have numDatapoint datapoints.
// Each dp will have numDimensions dimensions.
// Each metric will have the same unit, and value.
// But the value type will alternate between float and int.
// The metric data type will also alternative between gauge and sum.
// The timestamp on each datapoint will be the current time.
func createTestMetrics(
	numMetrics int,
	numDatapoints int,
	numDimensions int,
	unit string,
) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityType, "Service")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityDeploymentEnvironment, "MyEnvironment")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityServiceName, "MyServiceName")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityInstanceID, "i-123456789")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityAwsAccountId, "0123456789012")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityAutoScalingGroup, "asg-123")
	rm.Resource().Attributes().PutStr(entityattributes.AttributeEntityPlatformType, "AWS::EC2")

	sm := rm.ScopeMetrics().AppendEmpty()

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetDescription("my description")
		m.SetName(namePrefix + strconv.Itoa(i))
		m.SetUnit(unit)

		if i%2 == 0 {
			m.SetEmptyGauge()
		} else {
			m.SetEmptySum()
		}

		for j := 0; j < numDatapoints; j++ {
			var dp pmetric.NumberDataPoint
			if i%2 == 0 {
				dp = m.Gauge().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(metricValue))
			} else {
				dp = m.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(metricValue)
			}

			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			addDimensions(dp.Attributes(), numDimensions)
		}
	}

	return metrics
}

func createTestHistogram(
	numMetrics int,
	numDatapoints int,
	numDimensions int,
	unit string,
) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetDescription("my description")
		m.SetName(namePrefix + strconv.Itoa(i))
		m.SetUnit(unit)
		m.SetEmptyHistogram()
		for j := 0; j < numDatapoints; j++ {
			dp := m.Histogram().DataPoints().AppendEmpty()
			// Make the values match the count so it is easy to verify.
			dp.ExplicitBounds().Append(float64(1 + i))
			dp.ExplicitBounds().Append(float64(2 + 2*i))
			dp.BucketCounts().Append(uint64(1 + i))
			dp.BucketCounts().Append(uint64(2 + 2*i))
			dp.SetMax(histogramMax)
			dp.SetMin(histogramMin)
			dp.SetSum(histogramSum)
			dp.SetCount(histogramCount)
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			addDimensions(dp.Attributes(), numDimensions)
		}
	}
	return metrics
}

// checkDatum verifies unit, name, value, and dimension prefix.
func checkDatum(
	t *testing.T,
	d *aggregationDatum,
	unit string,
	numMetrics int,
) {
	assert.True(t, strings.HasPrefix(*d.MetricName, namePrefix))
	assert.Equal(t, unit, *d.Unit)
	if d.distribution == nil {
		// Verify single metric value.
		assert.Equal(t, metricValue, *d.Value)
	} else {
		// Verify distribution
		assert.Equal(t, float64(histogramMax), d.distribution.Maximum())
		assert.Equal(t, float64(histogramMin), d.distribution.Minimum())
		assert.Equal(t, float64(histogramSum), d.distribution.Sum())
		assert.Equal(t, float64(histogramCount), d.distribution.SampleCount())
		values, counts := d.distribution.ValuesAndCounts()
		assert.Equal(t, 2, len(values))
		assert.Equal(t, 2, len(counts))
		// Expect values and counts to match.
		// Refer to how createTestHistogram() sets them.
		assert.Equal(t, values[0], counts[0])
		assert.Equal(t, values[1], counts[1])
	}

	// Assuming unit test does not take more than 1 s.
	assert.Less(t, time.Since(*d.Timestamp), time.Second)
	for _, dim := range d.Dimensions {
		assert.True(t, strings.HasPrefix(*dim.Name, keyPrefix))
		assert.True(t, strings.HasPrefix(*dim.Value, valPrefix))
	}
}

func TestConvertOtelMetrics_NoDimensions(t *testing.T) {
	for i := 0; i < 100; i++ {
		metrics := createTestMetrics(i, i, 0, "Bytes")
		datums := ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i*i, len(datums))

		for _, d := range datums {
			assert.Equal(t, 0, len(d.Dimensions))
			checkDatum(t, d, "Bytes", i)

		}
	}
}

func TestConvertOtelMetrics_Histogram(t *testing.T) {
	for i := 0; i < 5; i++ {
		if i%2 == 0 {
			//distribution.NewDistribution = seh1.NewSEH1Distribution
			distribution.NewDistribution = regular.NewRegularDistribution
		} else {
			distribution.NewDistribution = regular.NewRegularDistribution
		}
		metrics := createTestHistogram(i, i, 0, "Bytes")
		datums := ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i*i, len(datums))

		// Verify dimensions per metric.
		for _, d := range datums {
			assert.Equal(t, 0, len(d.Dimensions))
			checkDatum(t, d, "Bytes", i)
		}
	}
}

func TestConvertOtelMetrics_Dimensions(t *testing.T) {
	for i := 0; i < 100; i++ {
		// 1 data point per metric, but vary the number dimensions.
		metrics := createTestMetrics(i, 1, i, "s")
		datums := ConvertOtelMetrics(metrics)
		// Expect nummetrics * numDatapointsPerMetric
		assert.Equal(t, i, len(datums))

		// Verify dimensions per metric.
		for _, d := range datums {
			expected := i
			if expected > 30 {
				expected = 30
			}
			assert.Equal(t, expected, len(d.Dimensions))
			checkDatum(t, d, "Seconds", i)
		}
	}
}

func TestConvertOtelMetrics_Entity(t *testing.T) {
	metrics := createTestMetrics(1, 1, 1, "s")
	datums := ConvertOtelMetrics(metrics)
	expectedEntity := cloudwatch.Entity{
		KeyAttributes: map[string]*string{
			"Type":         aws.String("Service"),
			"Environment":  aws.String("MyEnvironment"),
			"Name":         aws.String("MyServiceName"),
			"AwsAccountId": aws.String("0123456789012"),
		},
		Attributes: map[string]*string{
			"EC2.InstanceId":       aws.String("i-123456789"),
			"PlatformType":         aws.String("AWS::EC2"),
			"EC2.AutoScalingGroup": aws.String("asg-123"),
		},
	}
	assert.Equal(t, 1, len(datums))
	assert.Equal(t, expectedEntity, datums[0].entity)

}

func TestProcessAndRemoveEntityAttributes(t *testing.T) {
	testCases := []struct {
		name               string
		resourceAttributes map[string]any
		wantedAttributes   map[string]*string
		leftoverAttributes map[string]any
	}{
		{
			name: "key_attributes",
			resourceAttributes: map[string]any{
				entityattributes.AttributeEntityServiceName:           "my-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "my-environment",
			},
			wantedAttributes: map[string]*string{
				entityattributes.ServiceName:           aws.String("my-service"),
				entityattributes.DeploymentEnvironment: aws.String("my-environment"),
			},
			leftoverAttributes: make(map[string]any),
		},
		{
			name: "non-key_attributes",
			resourceAttributes: map[string]any{
				entityattributes.AttributeEntityCluster:      "my-cluster",
				entityattributes.AttributeEntityNamespace:    "my-namespace",
				entityattributes.AttributeEntityNode:         "my-node",
				entityattributes.AttributeEntityWorkload:     "my-workload",
				entityattributes.AttributeEntityPlatformType: "AWS::EKS",
			},
			wantedAttributes: map[string]*string{
				entityattributes.EksCluster:     aws.String("my-cluster"),
				entityattributes.NamespaceField: aws.String("my-namespace"),
				entityattributes.Node:           aws.String("my-node"),
				entityattributes.Workload:       aws.String("my-workload"),
				entityattributes.Platform:       aws.String("AWS::EKS"),
			},
			leftoverAttributes: make(map[string]any),
		},
		{
			name: "key_and_non_key_attributes",
			resourceAttributes: map[string]any{
				entityattributes.AttributeEntityServiceName:           "my-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "my-environment",
				entityattributes.AttributeEntityCluster:               "my-cluster",
				entityattributes.AttributeEntityNamespace:             "my-namespace",
				entityattributes.AttributeEntityNode:                  "my-node",
				entityattributes.AttributeEntityWorkload:              "my-workload",
				entityattributes.AttributeEntityPlatformType:          "K8s",
			},
			wantedAttributes: map[string]*string{
				entityattributes.ServiceName:           aws.String("my-service"),
				entityattributes.DeploymentEnvironment: aws.String("my-environment"),
				entityattributes.K8sCluster:            aws.String("my-cluster"),
				entityattributes.NamespaceField:        aws.String("my-namespace"),
				entityattributes.Node:                  aws.String("my-node"),
				entityattributes.Workload:              aws.String("my-workload"),
				entityattributes.Platform:              aws.String("K8s"),
			},
			leftoverAttributes: make(map[string]any),
		},
		{
			name: "key_and_non_key_attributes_plus_extras",
			resourceAttributes: map[string]any{
				"extra_attribute": "extra_value",
				entityattributes.AttributeEntityServiceName:           "my-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "my-environment",
				entityattributes.AttributeEntityCluster:               "my-cluster",
				entityattributes.AttributeEntityNamespace:             "my-namespace",
				entityattributes.AttributeEntityNode:                  "my-node",
				entityattributes.AttributeEntityWorkload:              "my-workload",
				entityattributes.AttributeEntityPlatformType:          "K8s",
			},
			wantedAttributes: map[string]*string{
				entityattributes.ServiceName:           aws.String("my-service"),
				entityattributes.DeploymentEnvironment: aws.String("my-environment"),
				entityattributes.K8sCluster:            aws.String("my-cluster"),
				entityattributes.NamespaceField:        aws.String("my-namespace"),
				entityattributes.Node:                  aws.String("my-node"),
				entityattributes.Workload:              aws.String("my-workload"),
				entityattributes.Platform:              aws.String("K8s"),
			},
			leftoverAttributes: map[string]any{
				"extra_attribute": "extra_value",
			},
		},
		{
			name: "key_and_non_key_attributes_plus_unsupported_entity_field",
			resourceAttributes: map[string]any{
				entityattributes.AWSEntityPrefix + "not.real.values":  "unsupported",
				entityattributes.AttributeEntityServiceName:           "my-service",
				entityattributes.AttributeEntityDeploymentEnvironment: "my-environment",
				entityattributes.AttributeEntityCluster:               "my-cluster",
				entityattributes.AttributeEntityNamespace:             "my-namespace",
				entityattributes.AttributeEntityNode:                  "my-node",
				entityattributes.AttributeEntityWorkload:              "my-workload",
				entityattributes.AttributeEntityPlatformType:          "AWS::EKS",
			},
			wantedAttributes: map[string]*string{
				entityattributes.ServiceName:           aws.String("my-service"),
				entityattributes.DeploymentEnvironment: aws.String("my-environment"),
				entityattributes.EksCluster:            aws.String("my-cluster"),
				entityattributes.NamespaceField:        aws.String("my-namespace"),
				entityattributes.Node:                  aws.String("my-node"),
				entityattributes.Workload:              aws.String("my-workload"),
				entityattributes.Platform:              aws.String("AWS::EKS"),
			},
			leftoverAttributes: map[string]any{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			err := attrs.FromRaw(tc.resourceAttributes)

			// resetting fields for current test case
			entityAttrMap := []map[string]string{entityattributes.GetKeyAttributeEntityShortNameMap()}
			platformType := ""
			if platformTypeValue, ok := attrs.Get(entityattributes.AttributeEntityPlatformType); ok {
				platformType = platformTypeValue.Str()
			}
			if platformType != "" {
				delete(entityattributes.GetAttributeEntityShortNameMap(platformType), entityattributes.AttributeEntityCluster)
				entityAttrMap = append(entityAttrMap, entityattributes.GetAttributeEntityShortNameMap(platformType))
			}
			assert.Nil(t, err)
			targetMap := make(map[string]*string)
			for _, entityMap := range entityAttrMap {
				processEntityAttributes(entityMap, targetMap, attrs)
			}
			removeEntityFields(attrs)
			assert.Equal(t, tc.leftoverAttributes, attrs.AsRaw())
			assert.Equal(t, tc.wantedAttributes, targetMap)
		})
	}
}

func TestFetchEntityFields_WithoutAccountID(t *testing.T) {
	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityType, "Service")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityDeploymentEnvironment, "my-environment")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityServiceName, "my-service")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityNode, "my-node")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityCluster, "my-cluster")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityNamespace, "my-namespace")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityWorkload, "my-workload")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityPlatformType, "AWS::EKS")
	assert.Equal(t, 8, resourceMetrics.Resource().Attributes().Len())

	expectedEntity := cloudwatch.Entity{
		KeyAttributes: nil,
		Attributes:    nil,
	}
	entity := fetchEntityFields(resourceMetrics.Resource().Attributes())
	assert.Equal(t, 0, resourceMetrics.Resource().Attributes().Len())
	assert.Equal(t, expectedEntity, entity)
}

func TestFetchEntityFields_WithAccountID(t *testing.T) {
	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityType, "Service")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityDeploymentEnvironment, "my-environment")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityServiceName, "my-service")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityNode, "my-node")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityCluster, "my-cluster")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityNamespace, "my-namespace")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityWorkload, "my-workload")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityPlatformType, "AWS::EKS")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityAwsAccountId, "123456789")
	assert.Equal(t, 9, resourceMetrics.Resource().Attributes().Len())

	expectedEntity := cloudwatch.Entity{
		KeyAttributes: map[string]*string{
			entityattributes.EntityType:            aws.String(entityattributes.Service),
			entityattributes.ServiceName:           aws.String("my-service"),
			entityattributes.DeploymentEnvironment: aws.String("my-environment"),
			entityattributes.AwsAccountId:          aws.String("123456789"),
		},
		Attributes: map[string]*string{
			entityattributes.Node:           aws.String("my-node"),
			entityattributes.EksCluster:     aws.String("my-cluster"),
			entityattributes.NamespaceField: aws.String("my-namespace"),
			entityattributes.Workload:       aws.String("my-workload"),
			entityattributes.Platform:       aws.String("AWS::EKS"),
		},
	}
	entity := fetchEntityFields(resourceMetrics.Resource().Attributes())
	assert.Equal(t, 0, resourceMetrics.Resource().Attributes().Len())
	assert.Equal(t, expectedEntity, entity)
}

func TestFetchEntityFieldsOnK8s(t *testing.T) {
	entityMap := entityattributes.GetAttributeEntityShortNameMap("")
	delete(entityMap, entityattributes.AttributeEntityCluster)
	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityType, "Service")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityDeploymentEnvironment, "my-environment")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityServiceName, "my-service")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityNode, "my-node")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityCluster, "my-cluster")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityNamespace, "my-namespace")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityWorkload, "my-workload")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityPlatformType, "K8s")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityAwsAccountId, "123456789")
	assert.Equal(t, 9, resourceMetrics.Resource().Attributes().Len())

	expectedEntity := cloudwatch.Entity{
		KeyAttributes: map[string]*string{
			entityattributes.EntityType:            aws.String(entityattributes.Service),
			entityattributes.ServiceName:           aws.String("my-service"),
			entityattributes.DeploymentEnvironment: aws.String("my-environment"),
			entityattributes.AwsAccountId:          aws.String("123456789"),
		},
		Attributes: map[string]*string{
			entityattributes.Node:           aws.String("my-node"),
			entityattributes.K8sCluster:     aws.String("my-cluster"),
			entityattributes.NamespaceField: aws.String("my-namespace"),
			entityattributes.Workload:       aws.String("my-workload"),
			entityattributes.Platform:       aws.String("K8s"),
		},
	}
	entity := fetchEntityFields(resourceMetrics.Resource().Attributes())
	assert.Equal(t, 0, resourceMetrics.Resource().Attributes().Len())
	assert.Equal(t, expectedEntity, entity)
}

func TestFetchEntityFieldsOnEc2(t *testing.T) {
	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityType, "Service")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityDeploymentEnvironment, "my-environment")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityServiceName, "my-service")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityPlatformType, "AWS::EC2")
	resourceMetrics.Resource().Attributes().PutStr(entityattributes.AttributeEntityAwsAccountId, "123456789")
	assert.Equal(t, 5, resourceMetrics.Resource().Attributes().Len())

	expectedEntity := cloudwatch.Entity{
		KeyAttributes: map[string]*string{
			entityattributes.EntityType:            aws.String(entityattributes.Service),
			entityattributes.ServiceName:           aws.String("my-service"),
			entityattributes.DeploymentEnvironment: aws.String("my-environment"),
			entityattributes.AwsAccountId:          aws.String("123456789"),
		},
		Attributes: map[string]*string{
			entityattributes.Platform: aws.String("AWS::EC2"),
		},
	}
	entity := fetchEntityFields(resourceMetrics.Resource().Attributes())
	assert.Equal(t, 0, resourceMetrics.Resource().Attributes().Len())
	assert.Equal(t, expectedEntity, entity)
}

func TestInvalidMetric(t *testing.T) {
	m := pmetric.NewMetric()
	m.SetName("name")
	m.SetUnit("unit")
	assert.Empty(t, ConvertOtelMetric(m, cloudwatch.Entity{}))
}
