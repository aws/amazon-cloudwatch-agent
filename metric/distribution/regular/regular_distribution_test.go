// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package regular

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"
)

var filenameReplacer = strings.NewReplacer(
	" ", "_",
	"/", "_",
)

func TestRegularDistribution(t *testing.T) {
	//dist new and add entry
	dist := NewRegularDistribution()

	assert.NoError(t, dist.AddEntry(20, 1))
	assert.NoError(t, dist.AddEntry(30, 1))
	assert.NoError(t, dist.AddEntryWithUnit(50, 1, "Count"))

	assert.Equal(t, 100.0, dist.Sum())
	assert.Equal(t, 3.0, dist.SampleCount())
	assert.Equal(t, 20.0, dist.Minimum())
	assert.Equal(t, 50.0, dist.Maximum())
	assert.Equal(t, "Count", dist.Unit())
	values, counts := dist.ValuesAndCounts()
	assert.Equal(t, len(values), len(counts))
	valuesCountsMap := map[float64]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[values[i]] = counts[i]
	}
	expectedValuesCountsMap := map[float64]float64{20: 1, 30: 1, 50: 1}
	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//another dist new and add entry
	anotherDist := NewRegularDistribution()

	assert.NoError(t, anotherDist.AddEntry(21, 1))
	assert.NoError(t, anotherDist.AddEntry(22, 1))
	assert.NoError(t, anotherDist.AddEntry(23, 2))

	assert.Equal(t, 89.0, anotherDist.Sum())
	assert.Equal(t, 4.0, anotherDist.SampleCount())
	assert.Equal(t, 21.0, anotherDist.Minimum())
	assert.Equal(t, 23.0, anotherDist.Maximum())
	assert.Equal(t, "", anotherDist.Unit())
	values, counts = anotherDist.ValuesAndCounts()
	assert.Equal(t, len(values), len(counts))
	valuesCountsMap = map[float64]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[values[i]] = counts[i]
	}
	expectedValuesCountsMap = map[float64]float64{21: 1, 22: 1, 23: 2}
	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//clone dist and anotherDist
	distClone := cloneRegularDistribution(dist.(*RegularDistribution))

	//add another dist into dist
	dist.AddDistribution(anotherDist)

	assert.Equal(t, 189.0, dist.Sum())
	assert.Equal(t, 7.0, dist.SampleCount())
	assert.Equal(t, 20.0, dist.Minimum())
	assert.Equal(t, 50.0, dist.Maximum())
	assert.Equal(t, "Count", dist.Unit())
	values, counts = dist.ValuesAndCounts()
	assert.Equal(t, len(values), len(counts))
	valuesCountsMap = map[float64]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[values[i]] = counts[i]
	}
	expectedValuesCountsMap = map[float64]float64{20: 1, 21: 1, 22: 1, 23: 2, 30: 1, 50: 1}
	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//add distClone into another dist
	anotherDist.AddDistribution(distClone)
	assert.Equal(t, dist, anotherDist) //the direction of AddDistribution should not matter.

	assert.ErrorIs(t, anotherDist.AddEntry(1, 0), distribution.ErrUnsupportedWeight)
	assert.ErrorIs(t, anotherDist.AddEntry(-1, 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(math.NaN(), 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(math.Inf(1), 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(math.Inf(-1), 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(distribution.MaxValue*1.001, 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(distribution.MinValue*1.001, 1), distribution.ErrUnsupportedValue)
}

func TestOutputOriginal(t *testing.T) {
	for _, tc := range histograms.TestCases() {
		jsonData, err := json.MarshalIndent(tc.Input, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile("testdata/original/"+filenameReplacer.Replace(tc.Name)+".json", jsonData, 0644))
	}
}

func TestCWAgent(t *testing.T) {
	t.Skip("intentionally does not pass")

	for _, tc := range histograms.TestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			dp := setupDatapoint(tc.Input)

			dist := NewFromOtelCWAgent(dp)
			fmt.Printf("%+v\n", dist)

			verifyDist(t, dist, tc.Expected)
			writeValuesAndCountsToJson(dist, "testdata/cwagent/"+filenameReplacer.Replace(tc.Name)+".json")
		})
	}

	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}
	tests := []struct {
		name        string
		filename    string
		newDistFunc func(pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts
	}{
		{
			name:        "lognormal",
			filename:    "testdata/lognormal_10000.csv",
			newDistFunc: NewFromOtelCWAgent,
		},
		{
			name:        "weibull",
			filename:    "testdata/weibull_10000.csv",
			newDistFunc: NewFromOtelCWAgent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := loadCsvData(tt.filename)
			require.NoError(t, err)
			assert.Len(t, data, 10000)

			dp := createHistogramFromData(data, boundaries)
			assert.Equal(t, int(dp.Count()), 10000)
			calculatedTotal := 0
			for _, count := range dp.BucketCounts().All() {
				calculatedTotal += int(count)
			}
			assert.Equal(t, calculatedTotal, 10000)

			dist := tt.newDistFunc(dp)
			writeValuesAndCountsToJson(dist, "testdata/cwagent/"+filenameReplacer.Replace(tt.name)+".json")
		})
	}

	t.Run("accuracy test - lognormal", func(t *testing.T) {
		verifyDistAccuracy(t, NewFromOtelCWAgent, "testdata/lognormal_10000.csv")
	})

	t.Run("accuracy test - weibull", func(t *testing.T) {
		verifyDistAccuracy(t, NewFromOtelCWAgent, "testdata/weibull_10000.csv")
	})

}

func TestMiddlePointMapping(t *testing.T) {
	t.Skip("intentionally does not pass")

	for _, tc := range histograms.TestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			dp := setupDatapoint(tc.Input)

			dist := NewMidpointMappingFromOtel(dp)
			fmt.Printf("%+v\n", dist)

			verifyDist(t, dist, tc.Expected)
			writeValuesAndCountsToJson(dist, "testdata/middlepoint/"+filenameReplacer.Replace(tc.Name)+".json")
		})
	}

	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}
	tests := []struct {
		name        string
		filename    string
		newDistFunc func(pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts
	}{
		{
			name:        "lognormal",
			filename:    "testdata/lognormal_10000.csv",
			newDistFunc: NewMidpointMappingFromOtel,
		},
		{
			name:        "weibull",
			filename:    "testdata/weibull_10000.csv",
			newDistFunc: NewMidpointMappingFromOtel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := loadCsvData(tt.filename)
			require.NoError(t, err)
			assert.Len(t, data, 10000)

			dp := createHistogramFromData(data, boundaries)
			assert.Equal(t, int(dp.Count()), 10000)
			calculatedTotal := 0
			for _, count := range dp.BucketCounts().All() {
				calculatedTotal += int(count)
			}
			assert.Equal(t, calculatedTotal, 10000)

			dist := tt.newDistFunc(dp)
			writeValuesAndCountsToJson(dist, "testdata/middlepoint/"+filenameReplacer.Replace(tt.name)+".json")
		})
	}

	t.Run("accuracy test - lognormal", func(t *testing.T) {
		verifyDistAccuracy(t, NewMidpointMappingFromOtel, "testdata/lognormal_10000.csv")
	})

	t.Run("accuracy test - weibull", func(t *testing.T) {
		verifyDistAccuracy(t, NewMidpointMappingFromOtel, "testdata/weibull_10000.csv")
	})

}

func TestEvenMapping(t *testing.T) {
	t.Skip("intentionally does not pass")

	for _, tc := range histograms.TestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			dp := setupDatapoint(tc.Input)

			dist := NewEvenMappingFromOtel(dp)
			//fmt.Printf("%+v\n", dist)

			verifyDist(t, dist, tc.Expected)
			writeValuesAndCountsToJson(dist, "testdata/even/"+filenameReplacer.Replace(tc.Name)+".json")
		})
	}

	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}
	tests := []struct {
		name        string
		filename    string
		newDistFunc func(pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts
	}{
		{
			name:        "lognormal",
			filename:    "testdata/lognormal_10000.csv",
			newDistFunc: NewEvenMappingFromOtel,
		},
		{
			name:        "weibull",
			filename:    "testdata/weibull_10000.csv",
			newDistFunc: NewEvenMappingFromOtel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := loadCsvData(tt.filename)
			require.NoError(t, err)
			assert.Len(t, data, 10000)

			dp := createHistogramFromData(data, boundaries)
			assert.Equal(t, int(dp.Count()), 10000)
			calculatedTotal := 0
			for _, count := range dp.BucketCounts().All() {
				calculatedTotal += int(count)
			}
			assert.Equal(t, calculatedTotal, 10000)

			dist := tt.newDistFunc(dp)
			writeValuesAndCountsToJson(dist, "testdata/even/"+filenameReplacer.Replace(tt.name)+".json")
		})
	}

	t.Run("accuracy test - lognormal", func(t *testing.T) {
		verifyDistAccuracy(t, NewEvenMappingFromOtel, "testdata/lognormal_10000.csv")
	})

	t.Run("accuracy test - weibull", func(t *testing.T) {
		verifyDistAccuracy(t, NewEvenMappingFromOtel, "testdata/weibull_10000.csv")
	})

}

func TestExponentialMapping(t *testing.T) {
	t.Skip("intentionally does not pass")

	for _, tc := range histograms.TestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			dp := setupDatapoint(tc.Input)

			dist := NewExponentialMappingFromOtel(dp)
			//fmt.Printf("%+v\n", dist)

			verifyDist(t, dist, tc.Expected)
			assert.NoError(t, writeValuesAndCountsToJson(dist, "testdata/exponential/"+filenameReplacer.Replace(tc.Name)+".json"))
		})
	}

	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}
	tests := []struct {
		name        string
		filename    string
		newDistFunc func(pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts
	}{
		{
			name:        "lognormal",
			filename:    "testdata/lognormal_10000.csv",
			newDistFunc: NewExponentialMappingFromOtel,
		},
		{
			name:        "weibull",
			filename:    "testdata/weibull_10000.csv",
			newDistFunc: NewExponentialMappingFromOtel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := loadCsvData(tt.filename)
			require.NoError(t, err)
			assert.Len(t, data, 10000)

			dp := createHistogramFromData(data, boundaries)
			assert.Equal(t, int(dp.Count()), 10000)
			calculatedTotal := 0
			for _, count := range dp.BucketCounts().All() {
				calculatedTotal += int(count)
			}
			assert.Equal(t, calculatedTotal, 10000)

			dist := tt.newDistFunc(dp)
			writeValuesAndCountsToJson(dist, "testdata/exponential/"+filenameReplacer.Replace(tt.name)+".json")
		})
	}

	t.Run("accuracy test - lognormal", func(t *testing.T) {
		verifyDistAccuracy(t, NewExponentialMappingFromOtel, "testdata/lognormal_10000.csv")
	})

	t.Run("accuracy test - weibull", func(t *testing.T) {
		verifyDistAccuracy(t, NewExponentialMappingFromOtel, "testdata/weibull_10000.csv")
	})
}

func TestExponentialMappingCW(t *testing.T) {

	for _, tc := range histograms.TestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			dp := setupDatapoint(tc.Input)

			dist := histograms.ConvertOTelToCloudWatch(dp)

			verifyDist(t, dist, tc.Expected)
			assert.NoError(t, writeValuesAndCountsToJson(dist, "testdata/exponentialcw/"+filenameReplacer.Replace(tc.Name)+".json"))
		})
	}

	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}
	tests := []struct {
		name        string
		filename    string
		newDistFunc func(pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts
	}{
		{
			name:        "lognormal",
			filename:    "testdata/lognormal_10000.csv",
			newDistFunc: NewExponentialMappingCWFromOtel,
		},
		{
			name:        "weibull",
			filename:    "testdata/weibull_10000.csv",
			newDistFunc: NewExponentialMappingCWFromOtel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := loadCsvData(tt.filename)
			require.NoError(t, err)
			assert.Len(t, data, 10000)

			dp := createHistogramFromData(data, boundaries)
			assert.Equal(t, int(dp.Count()), 10000)
			calculatedTotal := 0
			for _, count := range dp.BucketCounts().All() {
				calculatedTotal += int(count)
			}
			assert.Equal(t, calculatedTotal, 10000)

			dist := tt.newDistFunc(dp)
			writeValuesAndCountsToJson(dist, "testdata/exponentialcw/"+filenameReplacer.Replace(tt.name)+".json")
		})
	}

	t.Run("accuracy test - lognormal", func(t *testing.T) {
		verifyDistAccuracy(t, NewExponentialMappingCWFromOtel, "testdata/lognormal_10000.csv")
	})

	t.Run("accuracy test - weibull", func(t *testing.T) {
		verifyDistAccuracy(t, NewExponentialMappingCWFromOtel, "testdata/weibull_10000.csv")
	})

}

func BenchmarkLogNormal(b *testing.B) {
	// arrange
	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}

	data, err := loadCsvData("testdata/lognormal_10000.csv")
	require.NoError(b, err)
	require.Len(b, data, 10000)

	dp := createHistogramFromData(data, boundaries)
	require.Equal(b, int(dp.Count()), 10000)

	// b.Run("NewFromOtelCWAgent", func(b *testing.B) {
	// 	for i := 0; i < b.N; i++ {
	// 		dist := NewFromOtelCWAgent(dp)
	// 		values, counts := dist.ValuesAndCounts()
	// 		assert.NotNil(b, values)
	// 		assert.NotNil(b, counts)
	// 	}
	// })

	b.Run("NewExponentialMappingFromOtel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dist := NewExponentialMappingFromOtel(dp)
			values, counts := dist.ValuesAndCounts()
			assert.NotNil(b, values)
			assert.NotNil(b, counts)
		}
	})

	b.Run("NewExponentialMappingCWFromOtel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dist := NewExponentialMappingCWFromOtel(dp)
			values, counts := dist.ValuesAndCounts()
			assert.NotNil(b, values)
			assert.NotNil(b, counts)
		}
	})

}

func BenchmarkWeibull(b *testing.B) {
	// arrange
	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}

	data, err := loadCsvData("testdata/weibull_10000.csv")
	require.NoError(b, err)
	require.Len(b, data, 10000)

	dp := createHistogramFromData(data, boundaries)
	require.Equal(b, int(dp.Count()), 10000)

	b.Run("NewFromOtelCWAgent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dist := NewFromOtelCWAgent(dp)
			values, counts := dist.ValuesAndCounts()
			assert.NotNil(b, values)
			assert.NotNil(b, counts)
		}
	})

	b.Run("NewExponentialMappingFromOtel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dist := NewExponentialMappingFromOtel(dp)
			values, counts := dist.ValuesAndCounts()
			assert.NotNil(b, values)
			assert.NotNil(b, counts)
		}
	})

	b.Run("NewExponentialMappingCWFromOtel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dist := NewExponentialMappingCWFromOtel(dp)
			values, counts := dist.ValuesAndCounts()
			assert.NotNil(b, values)
			assert.NotNil(b, counts)
		}
	})

}

func BenchmarkExponentialMappingCW(b *testing.B) {
	// arrange
	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}

	data, err := loadCsvData("testdata/lognormal_10000.csv")
	require.NoError(b, err)
	assert.Len(b, data, 10000)

	dp := createHistogramFromData(data, boundaries)
	assert.Equal(b, int(dp.Count()), 10000)

	b.ResetTimer()

	// act
	for i := 0; i < b.N; i++ {
		dist := NewExponentialMappingCWFromOtel(dp)
		values, counts := dist.ValuesAndCounts()
		assert.NotNil(b, values)
		assert.NotNil(b, counts)
	}

}

func cloneRegularDistribution(dist *RegularDistribution) *RegularDistribution {
	clonedDist := &RegularDistribution{
		maximum:     dist.maximum,
		minimum:     dist.minimum,
		sampleCount: dist.sampleCount,
		sum:         dist.sum,
		buckets:     map[float64]float64{},
		unit:        dist.unit,
	}
	for k, v := range dist.buckets {
		clonedDist.buckets[k] = v
	}
	return clonedDist
}

func setupDatapoint(input histograms.HistogramInput) pmetric.HistogramDataPoint {
	dp := pmetric.NewHistogramDataPoint()
	dp.SetCount(input.Count)
	dp.SetSum(input.Sum)
	if input.Min != nil {
		dp.SetMin(*input.Min)
	}
	if input.Max != nil {
		dp.SetMax(*input.Max)
	}
	dp.ExplicitBounds().FromRaw(input.Boundaries)
	dp.BucketCounts().FromRaw(input.Counts)
	return dp
}

func verifyDist(t *testing.T, dist ToCloudWatchValuesAndCounts, expected histograms.ExpectedMetrics) {

	if expected.Min != nil {
		assert.Equal(t, *expected.Min, dist.Minimum(), "min does not match expected")
	}
	if expected.Max != nil {
		assert.Equal(t, *expected.Max, dist.Maximum(), "max does not match expected")
	}
	assert.Equal(t, int(expected.Count), int(dist.SampleCount()), "samplecount does not match expected")
	assert.Equal(t, expected.Sum, dist.Sum(), "sum does not match expected")

	values, counts := dist.ValuesAndCounts()

	calculatedCount := 0.0
	for _, count := range counts {
		calculatedCount += count
		//fmt.Printf("%7.2f = %4d (%d)\n", values[i], int(counts[i]), calculatedCount)
	}
	assert.InDelta(t, float64(expected.Count), calculatedCount, 1e-6, "calculated count does not match expected")

	for p, r := range expected.PercentileRanges {
		x := int(math.Round(float64(dist.SampleCount()) * p))

		soFar := 0
		for i, count := range counts {
			soFar += int(count)
			if soFar >= x {
				//fmt.Printf("Found p%.f at bucket %0.2f. Expected range: %+v\n", p*100, values[i], r)
				assert.GreaterOrEqual(t, values[i], r.Low, "percentile %0.2f", p)
				assert.LessOrEqual(t, values[i], r.High, "percentile %0.2f", p)
				break
			}
		}
	}
}

func loadCsvData(filename string) ([]float64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var data []float64
	for _, value := range records[0] {
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return nil, err
		}
		data = append(data, f)
	}
	return data, nil
}

func createHistogramFromData(data []float64, boundaries []float64) pmetric.HistogramDataPoint {
	dp := pmetric.NewHistogramDataPoint()

	// Calculate basic stats
	var sum float64
	min := math.Inf(1)
	max := math.Inf(-1)

	for _, v := range data {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	dp.SetCount(uint64(len(data)))
	dp.SetSum(sum)
	dp.SetMin(min)
	dp.SetMax(max)

	// Create bucket counts
	bucketCounts := make([]uint64, len(boundaries)+1)

	for _, v := range data {
		bucket := sort.SearchFloat64s(boundaries, v)
		bucketCounts[bucket]++
	}

	dp.ExplicitBounds().FromRaw(boundaries)
	dp.BucketCounts().FromRaw(bucketCounts)

	return dp
}

func verifyDistAccuracy(t *testing.T, newDistFunc func(pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts, filename string) {
	// arrange
	percentiles := []float64{0.1, 0.25, 0.5, 0.75, 0.9, 0.99, 0.999}
	boundaries := []float64{
		0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.01,
		0.011, 0.012, 0.013, 0.014, 0.015, 0.016, 0.017, 0.018, 0.019, 0.02,
		0.021, 0.022, 0.023, 0.024, 0.025, 0.026, 0.027, 0.028, 0.029, 0.03,
		0.031, 0.032, 0.033, 0.034, 0.035, 0.036, 0.037, 0.038, 0.039, 0.04,
		0.041, 0.042, 0.043, 0.044, 0.045, 0.046, 0.047, 0.048, 0.049, 0.05,
		0.1, 0.2,
	}

	data, err := loadCsvData(filename)
	require.NoError(t, err)
	assert.Len(t, data, 10000)

	dp := createHistogramFromData(data, boundaries)
	assert.Equal(t, int(dp.Count()), 10000)
	calculatedTotal := 0
	for _, count := range dp.BucketCounts().All() {
		calculatedTotal += int(count)
	}
	assert.Equal(t, calculatedTotal, 10000)

	// act
	dist := newDistFunc(dp)
	values, counts := dist.ValuesAndCounts()

	// assert
	calculatedCount := 0.0
	for _, count := range counts {
		calculatedCount += count
	}
	assert.InDelta(t, 10000, calculatedCount, 1e-6, "calculated count does not match expected")

	for _, p := range percentiles {
		x1 := int(math.Round(float64(dp.Count()) * p))
		x2 := int(math.Round(calculatedCount * p))

		exactPercentileValue := data[x1]

		soFar := 0
		for i, count := range counts {
			soFar += int(count)
			if soFar >= x2 {
				calculatedPercentileValue := values[i]
				errorPercent := (exactPercentileValue - calculatedPercentileValue) / exactPercentileValue * 100
				fmt.Printf("P%.1f: exact=%.6f, calculated=%.6f, error=%.2f%%\n", p*100, exactPercentileValue, calculatedPercentileValue, errorPercent)
				break
			}

		}

	}
}

func writeValuesAndCountsToJson(dist ToCloudWatchValuesAndCounts, filename string) error {
	values, counts := dist.ValuesAndCounts()

	data := make(map[string][]float64)
	data["values"] = values
	data["counts"] = counts

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, jsonData, 0644)
}
