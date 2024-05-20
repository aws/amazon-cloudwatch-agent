// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cardinalitycontrol

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	awsapplicationsignalsconfig "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
)

var emptyResourceAttributes = pcommon.NewMap()
var logger, _ = zap.NewDevelopment()

func TestAdmitAndRollup(t *testing.T) {
	config := &awsapplicationsignalsconfig.LimiterConfig{
		Threshold:         2,
		Disabled:          false,
		LogDroppedMetrics: false,
		RotationInterval:  awsapplicationsignalsconfig.DefaultRotationInterval,
	}
	config.Validate()

	limiter := NewMetricsLimiter(config, logger)

	admittedAttributes := map[string]pcommon.Map{}
	for i := 0; i < 10; i++ {
		attr := newLowCardinalityAttributes(100)
		if ok, _ := limiter.Admit("latency", attr, emptyResourceAttributes); ok {
			uniqKey, _ := attr.Get("RemoteOperation")
			admittedAttributes[uniqKey.AsString()] = attr
		} else {
			for _, indexedAttrKey := range awsDeclaredMetricAttributes {
				if indexedAttrKey == common.MetricAttributeEnvironment ||
					indexedAttrKey == common.MetricAttributeLocalService ||
					indexedAttrKey == common.MetricAttributeRemoteService {
					continue
				}
				attrValue, _ := attr.Get(indexedAttrKey)
				if indexedAttrKey == common.MetricAttributeLocalOperation {
					assert.Equal(t, UnprocessedServiceOperationValue, attrValue.AsString())
				} else if indexedAttrKey == common.MetricAttributeRemoteOperation {
					assert.Equal(t, UnprocessedRemoteServiceOperationValue, attrValue.AsString())
				} else {
					assert.Equal(t, "-", attrValue.AsString())
				}
			}
		}
	}
	assert.Equal(t, 2, len(admittedAttributes), fmt.Sprintf("admitted attributes are %v", admittedAttributes))
}

func TestAdmitByTopK(t *testing.T) {
	config := awsapplicationsignalsconfig.LimiterConfig{
		Threshold:         100,
		Disabled:          false,
		LogDroppedMetrics: false,
		RotationInterval:  awsapplicationsignalsconfig.DefaultRotationInterval,
	}
	config.Validate()

	limiter := NewMetricsLimiter(&config, logger)

	// fulfill topk with high cardinality attributes
	for i := 0; i < 110; i++ {
		attr := newHighCardinalityAttributes()
		limiter.Admit("latency", attr, emptyResourceAttributes)
	}

	// sending low cardinality attributes
	for i := 0; i < 100; i++ {
		attr := newFixedAttributes(i % 20)
		limiter.Admit("latency", attr, emptyResourceAttributes)
	}

	for i := 0; i < 20; i++ {
		attr := newFixedAttributes(i)
		ok, _ := limiter.Admit("latency", attr, emptyResourceAttributes)
		assert.True(t, ok)
	}
}

func TestAdmitLowCardinalityAttributes(t *testing.T) {
	config := awsapplicationsignalsconfig.LimiterConfig{
		Threshold:         10,
		Disabled:          false,
		LogDroppedMetrics: false,
		RotationInterval:  awsapplicationsignalsconfig.DefaultRotationInterval,
	}
	config.Validate()

	limiter := NewMetricsLimiter(&config, logger)

	rejectCount := 0
	for i := 0; i < 100; i++ {
		if ok, _ := limiter.Admit("latency", newLowCardinalityAttributes(10), emptyResourceAttributes); !ok {
			rejectCount += 1
		}
	}
	assert.Equal(t, 0, rejectCount)
}

func TestAdmitReservedMetrics(t *testing.T) {
	config := awsapplicationsignalsconfig.LimiterConfig{
		Threshold:         10,
		Disabled:          false,
		LogDroppedMetrics: false,
		RotationInterval:  awsapplicationsignalsconfig.DefaultRotationInterval,
	}
	config.Validate()

	limiter := NewMetricsLimiter(&config, logger)

	// fulfill topk with high cardinality attributes
	for i := 0; i < 20; i++ {
		attr := newHighCardinalityAttributes()
		limiter.Admit("latency", attr, emptyResourceAttributes)
	}

	for i := 0; i < 20; i++ {
		attr := newHighCardinalityAttributes()
		// simulate attributes touched by customization rules
		attr.PutBool(common.AttributeTmpReserved, true)

		ok, _ := limiter.Admit("latency", attr, emptyResourceAttributes)
		assert.True(t, ok)
		_, exists := attr.Get(common.AttributeTmpReserved)
		assert.False(t, exists)
	}
}

func TestClearStaleService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())

	config := awsapplicationsignalsconfig.LimiterConfig{
		Threshold:         10,
		Disabled:          false,
		LogDroppedMetrics: false,

		ParentContext:             ctx,
		RotationInterval:          time.Second,
		GarbageCollectionInterval: time.Second,
	}
	limiter := NewMetricsLimiter(&config, logger)

	for i := 0; i < 10; i++ {
		appName := "app" + strconv.Itoa(i)
		attr := pcommon.NewMap()
		attr.PutStr("Service", appName)
		limiter.Admit(appName, attr, emptyResourceAttributes)
	}

	time.Sleep(10 * time.Second)
	cancel()

	metricsLimiter := limiter.(*MetricsLimiter)
	assert.Equal(t, 0, len(metricsLimiter.services))
}

func TestInheritanceAfterRotation(t *testing.T) {
	config := awsapplicationsignalsconfig.LimiterConfig{
		Threshold:         10,
		Disabled:          false,
		LogDroppedMetrics: true,
		RotationInterval:  5 * time.Second,
	}
	config.Validate()

	limiter := NewMetricsLimiter(&config, logger)

	// fulfill primary with 0-10
	for i := 0; i < 10; i++ {
		attr := newFixedAttributes(i)
		ok, _ := limiter.Admit("latency", attr, emptyResourceAttributes)
		assert.True(t, ok)
	}

	// wait for rotation
	time.Sleep(6 * time.Second)
	// validate 0-10 are admitted
	for i := 0; i < 10; i++ {
		attr := newFixedAttributes(i)
		ok, _ := limiter.Admit("latency", attr, emptyResourceAttributes)
		assert.True(t, ok)
	}

	// validate 10-20 are rejected
	// promote 10-20 to top k
	for j := 0; j < 2; j++ {
		for i := 10; i < 20; i++ {
			attr := newFixedAttributes(i)
			ok, _ := limiter.Admit("latency", attr, emptyResourceAttributes)
			assert.False(t, ok)
		}
	}

	// wait for rotation
	time.Sleep(6 * time.Second)

	// validate 1--20 are admitted
	for i := 10; i < 20; i++ {
		attr := newFixedAttributes(i)
		ok, _ := limiter.Admit("latency", attr, emptyResourceAttributes)
		assert.True(t, ok)
	}
}

func TestRotationInterval(t *testing.T) {
	svc := newService("test", 1, 5*time.Second, context.Background(), logger)
	// wait for secondary to be created
	time.Sleep(7 * time.Second)
	for i := 0; i < 5; i++ {
		svc.secondaryCMS.matrix[0][0] = 1

		// wait for rotation
		time.Sleep(5 * time.Second)

		// verify secondary is promoted to primary
		assert.Equal(t, 0, svc.secondaryCMS.matrix[0][0])
		assert.Equal(t, 1, svc.primaryCMS.matrix[0][0])
	}
}

func newRandomIP() string {
	rand.NewSource(time.Now().UnixNano())

	ipPart1 := rand.Intn(256)
	ipPart2 := rand.Intn(256)
	ipPart3 := rand.Intn(256)
	ipPart4 := rand.Intn(256)

	return fmt.Sprintf("%d.%d.%d.%d", ipPart1, ipPart2, ipPart3, ipPart4)
}

func newFixedAttributes(val int) pcommon.Map {
	methodName := "/test" + strconv.Itoa(val)
	attr := pcommon.NewMap()
	attr.PutStr("Service", "app")
	attr.PutStr("Operation", "/api/gateway"+methodName)
	attr.PutStr("RemoteService", "upstream1")
	attr.PutStr("RemoteOperation", methodName)
	return attr
}

func newLowCardinalityAttributes(admitRange int) pcommon.Map {
	methodName := "/test" + strconv.Itoa(rand.Intn(admitRange))
	attr := pcommon.NewMap()
	attr.PutStr("Service", "app")
	attr.PutStr("Operation", "/api/gateway"+methodName)
	attr.PutStr("RemoteService", "upstream1")
	attr.PutStr("RemoteOperation", methodName)
	return attr
}

func newHighCardinalityAttributes() pcommon.Map {
	attr := pcommon.NewMap()
	attr.PutStr("Service", "app")
	attr.PutStr("Operation", "/api/gateway/test")
	attr.PutStr("RemoteService", newRandomIP())
	attr.PutStr("RemoteOperation", "/test/"+strconv.Itoa(rand.Int()))
	return attr
}
