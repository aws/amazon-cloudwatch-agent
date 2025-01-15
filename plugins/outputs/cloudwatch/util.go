// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/seh1"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatch"
)

const (
	maxValuesPerDatum = 5000

	// Constant for estimate encoded metric size
	// Action=PutMetricData
	pmdActionSize = 20
	// &Version=2010-08-01
	versionSize = 19
	// &MetricData.member.100.StatisticValues.Maximum=1558.3086995967291&MetricData.member.100.StatisticValues.Minimum=1558.3086995967291&MetricData.member.100.StatisticValues.SampleCount=1000&MetricData.member.100.StatisticValues.Sum=1558.3086995967291
	statisticsSize = 246
	// &MetricData.member.100.Timestamp=2018-05-29T21%3A14%3A00Z
	timestampSize = 57
	// &StrictEntityValidation=false
	strictEntityValidationSize = 29

	overallConstPerRequestSize = pmdActionSize + versionSize
	// &Namespace=, this is per request
	namespaceOverheads = 11

	// &MetricData.member.100.Dimensions.member.1.Name= &MetricData.member.100.Dimensions.member.1.Value=
	dimensionOverheads = 48 + 49
	// &MetricData.member.100.MetricName=
	metricNameOverheads = 34
	// &MetricData.member.100.StorageResolution=1
	highResolutionOverheads = 42
	// &MetricData.member.100.Values.member.100=1558.3086995967291 &MetricData.member.100.Counts.member.100=1000
	valuesCountsOverheads = 59 + 45
	// &MetricData.member.100.Value=1558.3086995967291
	valueOverheads = 47
	// &MetricData.member.1.Unit=Kilobytes/Second
	unitOverheads = 42
	// &EntityMetricData.member.100.Entity.KeyAttributes.entry.1.key= &EntityMetricData.member.100.Entity.KeyAttributes.entry.1.value=
	entityKeyAttributesOverhead = 62 + 64
	// &EntityMetricData.member.100.Entity.Attributes.entry.1.key= &EntityMetricData.member.100.Entity.Attributes.entry.1.value=
	entityAttributesOverhead = 59 + 61
	// EntityMetricData.member.100.
	entityMetricDataPrefixOverhead = 28
)

// Set seed once.
var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// publishJitter returns a random duration between 0 and the given publishInterval.
func publishJitter(publishInterval time.Duration) time.Duration {
	jitter := seededRand.Int63n(int64(publishInterval))
	return time.Duration(jitter)
}

func setNewDistributionFunc(maxValuesPerDatumLimit int) {
	if maxValuesPerDatumLimit >= maxValuesPerDatum {
		distribution.NewDistribution = seh1.NewSEH1Distribution
	} else {
		distribution.NewDistribution = regular.NewRegularDistribution
	}
}

func resize(dist distribution.Distribution, listMaxSize int) (distList []distribution.Distribution) {
	var ok bool
	// If this is SEH1 distribution, it has already considered the list max size.
	if _, ok = dist.(*seh1.SEH1Distribution); ok {
		distList = append(distList, dist)
		return
	}
	var regularDist *regular.RegularDistribution
	if regularDist, ok = dist.(*regular.RegularDistribution); !ok {
		log.Printf("E! The distribution type %T is not supported for resizing.", dist)
		return
	}
	values, _ := regularDist.ValuesAndCounts()
	sort.Float64s(values)
	newSEH1Dist := seh1.NewSEH1Distribution().(*seh1.SEH1Distribution)
	for i := 0; i < len(values); i++ {
		if !newSEH1Dist.CanAdd(values[i], listMaxSize) {
			distList = append(distList, newSEH1Dist)
			newSEH1Dist = seh1.NewSEH1Distribution().(*seh1.SEH1Distribution)
		}
		newSEH1Dist.AddEntry(values[i], regularDist.GetCount(values[i]))
	}
	if newSEH1Dist.Size() > 0 {
		distList = append(distList, newSEH1Dist)
	}
	return
}

func payload(datum *cloudwatch.MetricDatum, entityPresent bool) int {
	size := timestampSize
	// this multiplier keeps track of how many times we should be adding "EntityMetricData.member.100." to our payload
	entityPrefixMultiplier := 1

	for _, dimension := range datum.Dimensions {
		size += len(*dimension.Name) + len(*dimension.Value) + dimensionOverheads
		entityPrefixMultiplier += 2 // Each dimension has Name/Value so we add the entity string twice
	}

	if datum.MetricName != nil {
		// The metric name won't be nil, but it should fail in the validation instead of panic here.
		size += len(*datum.MetricName) + metricNameOverheads
		entityPrefixMultiplier++
	}

	if datum.StorageResolution != nil {
		size += highResolutionOverheads
		entityPrefixMultiplier++
	}

	valuesCountsLen := len(datum.Values)
	if valuesCountsLen != 0 {
		size += valuesCountsLen*valuesCountsOverheads + statisticsSize
		// Add the entity string twice for each Value/Count and then 4 more times for the statistic
		entityPrefixMultiplier += 2*valuesCountsLen + 4
	} else {
		size += valueOverheads
		entityPrefixMultiplier++
	}

	if datum.Unit != nil {
		size += unitOverheads
		entityPrefixMultiplier++
	}

	if entityPresent {
		size += entityPrefixMultiplier * entityMetricDataPrefixOverhead
	}

	return size
}

func calculateEntitySize(entity cloudwatch.Entity) int {
	size := strictEntityValidationSize

	for k, v := range entity.Attributes {
		size += len(k) + len(*v) + entityAttributesOverhead
	}

	for k, v := range entity.KeyAttributes {
		size += len(k) + len(*v) + entityKeyAttributesOverhead
	}

	return size
}

func entityToString(entity cloudwatch.Entity) string {
	var attributes, keyAttributes, data string
	if entity.Attributes != nil {
		attributes = entityAttributesToString(entity.Attributes)
	}
	if entity.KeyAttributes != nil {
		keyAttributes = entityAttributesToString(entity.KeyAttributes)
	}

	if attributes != "" || keyAttributes != "" {
		data = fmt.Sprintf(
			"%s|%s",
			attributes,
			keyAttributes,
		)
	}
	return data
}

// Helper function to convert a map of entityAttributes to a consistent string representation
func entityAttributesToString(m map[string]*string) string {
	if m == nil {
		return ""
	}
	pairs := make([]string, 0, len(m))
	for k, v := range m {
		if v == nil {
			pairs = append(pairs, k+":")
		} else {
			pairs = append(pairs, k+":"+*v)
		}
	}
	sort.Strings(pairs) // Ensure a consistent order
	return strings.Join(pairs, ";")
}

func stringToEntity(data string) cloudwatch.Entity {
	parts := strings.Split(data, "|")
	if len(parts) < 2 {
		// Handle error: invalid input string
		return cloudwatch.Entity{}
	}

	entity := cloudwatch.Entity{
		Attributes:    make(map[string]*string),
		KeyAttributes: make(map[string]*string),
	}

	if parts[0] != "" {
		entity.Attributes = stringToEntityAttributes(parts[0])
	}

	if parts[1] != "" {
		entity.KeyAttributes = stringToEntityAttributes(parts[1])
	}

	return entity
}

func stringToEntityAttributes(s string) map[string]*string {
	result := make(map[string]*string)
	pairs := strings.Split(s, ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) == 2 {
			value := kv[1]
			result[kv[0]] = &value
		}
	}
	return result
}
