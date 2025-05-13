// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package exph

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestSize(t *testing.T) {
	tests := []struct {
		name           string
		posBucketCount int
		negBucketCount int
		zeroCount      uint64
		expectedSize   int
	}{
		{
			name:           "positive buckets",
			posBucketCount: 100,
			expectedSize:   100,
		},
		{
			name:           "negative buckets",
			negBucketCount: 200,
			expectedSize:   200,
		},
		{
			name:         "zero bucket",
			zeroCount:    10,
			expectedSize: 1,
		},
		{
			name:           "positive and negative buckets",
			posBucketCount: 120,
			negBucketCount: 120,
			expectedSize:   240,
		},
		{
			name:           "positive and zero buckets",
			posBucketCount: 120,
			zeroCount:      80,
			expectedSize:   121,
		},
		{
			name:           "negative and zero buckets",
			negBucketCount: 16,
			zeroCount:      80,
			expectedSize:   17,
		},
		{
			name:           "positive, negative, and zero buckets",
			posBucketCount: 20,
			negBucketCount: 10,
			zeroCount:      80,
			expectedSize:   31,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exph := NewExpHistogramDistribution()
			exph.positiveBuckets = map[int]uint64{}
			exph.negativeBuckets = map[int]uint64{}
			exph.zeroCount = tt.zeroCount

			for i := 0; i < tt.posBucketCount; i++ {
				exph.positiveBuckets[i] = uint64(i + 1) //nolint:gosec
			}

			for i := range tt.negBucketCount {
				exph.negativeBuckets[i] = uint64(i + 1) //nolint:gosec
			}

			assert.Equal(t, tt.expectedSize, exph.Size())
		})
	}
}

func TestValuesAndCounts(t *testing.T) {

	tests := []struct {
		name           string
		histogram      *ExpHistogramDistribution
		expectedValues []float64
		expectedCounts []float64
	}{
		{
			name: "positive buckets",
			histogram: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.positiveBuckets = make(map[int]uint64, 10)
				for i := range 10 {
					exph.positiveBuckets[i] = uint64(i + 1) //nolint:gosec
				}
				return exph
			}(),
			expectedValues: []float64{
				768.0, 384.0, 192.0, 96.0, 48.0, 24.0, 12.0, 6.0, 3.0, 1.5,
			},
			expectedCounts: []float64{
				10, 9, 8, 7, 6, 5, 4, 3, 2, 1,
			},
		},
		{
			name: "positive buckets w/ some empty",
			histogram: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.positiveBuckets = make(map[int]uint64, 10)
				for i := range 10 {
					if i%2 == 0 {
						exph.positiveBuckets[i] = uint64(i + 1) //nolint:gosec
					}
				}
				return exph
			}(),
			expectedValues: []float64{
				384.0, 96.0, 24.0, 6.0, 1.5,
			},
			expectedCounts: []float64{
				9, 7, 5, 3, 1,
			},
		},
		{
			name: "negative buckets",
			histogram: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.negativeBuckets = make(map[int]uint64, 10)
				for i := range 10 {
					exph.negativeBuckets[i] = uint64(i + 1) //nolint:gosec
				}
				return exph
			}(),
			expectedValues: []float64{
				-1.5, -3.0, -6.0, -12.0, -24.0, -48.0, -96.0, -192.0, -384.0, -768.0,
			},
			expectedCounts: []float64{
				1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
			},
		},
		{
			name: "negative buckets w/ some empty",
			histogram: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.negativeBuckets = make(map[int]uint64, 10)
				for i := range 10 {
					if i%2 == 1 {
						exph.negativeBuckets[i] = uint64(i + 1) //nolint:gosec
					}
				}
				return exph
			}(),
			expectedValues: []float64{
				-3.0, -12.0, -48.0, -192.0, -768.0,
			},
			expectedCounts: []float64{
				2, 4, 6, 8, 10,
			},
		},
		{
			name: "zero bucket",
			histogram: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.zeroCount = 10
				return exph
			}(),
			expectedValues: []float64{
				0,
			},
			expectedCounts: []float64{
				10,
			},
		},
		{
			name: "positive, negative, and zero buckets",
			histogram: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.positiveBuckets = make(map[int]uint64, 120)
				exph.negativeBuckets = make(map[int]uint64, 120)
				for i := range 120 {
					exph.positiveBuckets[i] = uint64(i + 1) //nolint:gosec
				}
				exph.zeroCount = 10
				for i := range 120 {
					exph.negativeBuckets[i] = uint64(i + 1) //nolint:gosec
				}
				return exph
			}(),
			expectedValues: []float64{
				// Largest bucket should be positive bucket index 119 (zero-origin)
				// Start: 6.65e35 (2^119)
				// Mid:   9.97e35 (start+end)/2
				// End:   1.33e36 (2^120)
				//
				// Smallest bucket should be negative bucket index 28 (120 positive buckets + 1 zero-value bucket + 29 negative buckets)
				// Start: -2.67e8 (2^28)
				// Mid:   -4.03e8 (start+end)/2
				// End:   -5.37e8 (2^29)

				9.969209968386869e+35, 4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34, 3.1153781151208966e+34,
				1.5576890575604483e+34, 7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33, 9.735556609752802e+32, 4.867778304876401e+32,
				2.4338891524382005e+32, 1.2169445762191002e+32, 6.084722881095501e+31, 3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30,
				3.802951800684688e+30, 1.901475900342344e+30, 9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29,
				5.942112188569825e+28, 2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
				9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25, 2.90142196707511e+25,
				1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24, 9.066943647109719e+23, 4.5334718235548594e+23,
				2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22, 2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21,
				3.541774862152234e+21, 1.770887431076117e+21, 8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20,
				5.5340232221128655e+19, 2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
				8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16, 2.7021597764222976e+16,
				1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15, 8.44424930131968e+14, 4.22212465065984e+14,
				2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12,
				3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11, 4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10,
				2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08,
				1.00663296e+08, 5.0331648e+07, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304,
				49152, 24576, 12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768,
				-1536, -3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07,
				-2.5165824e+07, -5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -1.610612736e+09, -3.221225472e+09,
				-6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11,
				-8.24633720832e+11, -1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13,
				-5.2776558133248e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15,
				-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -5.404319552844595e+16,
				-1.080863910568919e+17, -2.161727821137838e+17, -4.323455642275676e+17, -8.646911284551352e+17, -1.7293822569102705e+18, -3.458764513820541e+18,
				-6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19, -5.5340232221128655e+19, -1.1068046444225731e+20,
				-2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20, -1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21,
				-1.4167099448608936e+22, -2.833419889721787e+22, -5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23,
				-4.5334718235548594e+23, -9.066943647109719e+23, -1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24, -1.450710983537555e+25,
				-2.90142196707511e+25, -5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26, -9.284550294640352e+26,
				-1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28, -2.9710560942849127e+28, -5.942112188569825e+28,
				-1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29, -9.50737950171172e+29, -1.901475900342344e+30, -3.802951800684688e+30,
				-7.605903601369376e+30, -1.5211807202738753e+31, -3.0423614405477506e+31, -6.084722881095501e+31, -1.2169445762191002e+32,
				-2.4338891524382005e+32, -4.867778304876401e+32, -9.735556609752802e+32, -1.9471113219505604e+33, -3.894222643901121e+33, -7.788445287802241e+33,
				-1.5576890575604483e+34, -3.1153781151208966e+34, -6.230756230241793e+34, -1.2461512460483586e+35, -2.4923024920967173e+35,
				-4.9846049841934345e+35, -9.969209968386869e+35,
			},
			expectedCounts: []float64{
				120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100, 99, 98, 97, 96, 95, 94, 93, 92,
				91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65, 64, 63, 62, 61, 60, 59, 58,
				57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28, 27, 26, 25, 24,
				23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14,
				15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
				49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82,
				83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113,
				114, 115, 116, 117, 118, 119, 120,
			},
		},
		{
			name: "positive scale",
			histogram: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 2
				exph.positiveBuckets = make(map[int]uint64, 10)
				for i := range 10 {
					exph.positiveBuckets[i] = uint64(i + 1) //nolint:gosec
				}
				exph.negativeBuckets = make(map[int]uint64, 10)
				for i := range 10 {
					exph.negativeBuckets[i] = uint64(i + 1) //nolint:gosec
				}
				return exph
			}(),
			expectedValues: []float64{
				5.2068413547516315, 4.378414230005442, 3.681792830507429, 3.0960063928805237, 2.6034206773758157, 2.189207115002721, 1.8408964152537144,
				1.5480031964402619, 1.3017103386879079, 1.0946035575013604, -1.0946035575013604, -1.3017103386879079, -1.5480031964402619, -1.8408964152537144,
				-2.189207115002721, -2.6034206773758157, -3.0960063928805237, -3.681792830507429, -4.378414230005442, -5.2068413547516315,
			},
			expectedCounts: []float64{
				10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
			},
		},
		{
			name: "negative scale",
			histogram: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = -3
				exph.positiveBuckets = make(map[int]uint64, 10)
				for i := range 10 {
					exph.positiveBuckets[i] = uint64(i + 1) //nolint:gosec
				}
				exph.negativeBuckets = make(map[int]uint64, 10)
				for i := range 10 {
					exph.negativeBuckets[i] = uint64(i + 1) //nolint:gosec
				}
				return exph
			}(),
			expectedValues: []float64{
				6.068240930487494e+23, 2.3704066134716774e+21, 9.25940083387374e+18, 3.61695345073193e+16, 1.41287244169216e+14, 5.51903297536e+11,
				2.155872256e+09, 8.421376e+06, 32896, 128.5, -128.5, -32896, -8.421376e+06, -2.155872256e+09, -5.51903297536e+11, -1.41287244169216e+14,
				-3.61695345073193e+16, -9.25940083387374e+18, -2.3704066134716774e+21, -6.068240930487494e+23,
			},
			expectedCounts: []float64{
				10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, counts := tt.histogram.ValuesAndCounts()
			assert.Len(t, values, len(tt.expectedValues))
			assert.Len(t, counts, len(tt.expectedCounts))
			assert.Equal(t, tt.expectedValues, values)
			assert.Equal(t, tt.expectedCounts, counts)
		})
	}

}

func TestAddDistribution(t *testing.T) {
	tests := []struct {
		name         string
		exph1        *ExpHistogramDistribution
		exph2        *ExpHistogramDistribution
		expectedExph *ExpHistogramDistribution
	}{
		{
			name: "zero bucket",
			exph1: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.max = 0
				exph.min = 0
				exph.sampleCount = 21
				exph.zeroCount = 21
				return exph
			}(),
			exph2: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.max = 0
				exph.min = 0
				exph.sampleCount = 15
				exph.zeroCount = 15
				return exph
			}(),
			expectedExph: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.scale = 0
				exph.max = 0
				exph.min = 0
				exph.sampleCount = 36
				exph.zeroCount = 36
				return exph
			}(),
		},
		{
			name: "positive, non-overlapping buckets",
			exph1: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 48
				exph.min = 2
				exph.sampleCount = 21
				exph.sum = 90
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6,
				}
				return exph
			}(),
			exph2: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 750
				exph.min = 96
				exph.sampleCount = 10
				exph.sum = 812
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					6: 1, 7: 2, 8: 3, 9: 4,
				}
				return exph
			}(),
			expectedExph: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 750
				exph.min = 2
				exph.sampleCount = 31
				exph.sum = 902
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 1, 7: 2, 8: 3, 9: 4,
				}
				return exph
			}(),
		},
		{
			name: "positive, overlapping buckets",
			exph1: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 1100
				exph.min = 2
				exph.sampleCount = 55
				exph.sum = 1300
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 9, 9: 10,
				}
				return exph
			}(),
			exph2: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 48
				exph.min = 1
				exph.sampleCount = 21
				exph.sum = 70
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6,
				}
				return exph
			}(),
			expectedExph: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 1100
				exph.min = 1
				exph.sampleCount = 76
				exph.scale = 0
				exph.sum = 1370
				exph.positiveBuckets = map[int]uint64{
					0: 2, 1: 4, 2: 6, 3: 8, 4: 10, 5: 12, 6: 7, 7: 8, 8: 9, 9: 10,
				}
				return exph
			}(),
		},
		{
			name: "negative, non-overlapping buckets",
			exph1: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = -2
				exph.min = -48
				exph.sampleCount = 21
				exph.sum = -70
				exph.scale = 0
				exph.negativeBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6,
				}
				return exph
			}(),
			exph2: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = -96
				exph.min = -750
				exph.sampleCount = 10
				exph.sum = -900
				exph.scale = 0
				exph.negativeBuckets = map[int]uint64{
					6: 1, 7: 2, 8: 3, 9: 4,
				}
				return exph
			}(),
			expectedExph: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = -2
				exph.min = -750
				exph.sampleCount = 31
				exph.scale = 0
				exph.sum = -970
				exph.negativeBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 1, 7: 2, 8: 3, 9: 4,
				}
				return exph
			}(),
		},
		{
			name: "negative, overlapping buckets",
			exph1: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = -2
				exph.min = -1100
				exph.sampleCount = 55
				exph.sum = -1300
				exph.scale = 0
				exph.negativeBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 9, 9: 10,
				}
				return exph
			}(),
			exph2: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = -1
				exph.min = -48
				exph.sampleCount = 21
				exph.sum = -70
				exph.scale = 0
				exph.negativeBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6,
				}
				return exph
			}(),
			expectedExph: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = -1
				exph.min = -1100
				exph.sampleCount = 76
				exph.sum = -1370
				exph.scale = 0
				exph.negativeBuckets = map[int]uint64{
					0: 2, 1: 4, 2: 6, 3: 8, 4: 10, 5: 12, 6: 7, 7: 8, 8: 9, 9: 10,
				}
				return exph
			}(),
		},
		{
			name: "positive and negative, non-overlapping",
			exph1: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 1100
				exph.min = 2
				exph.sampleCount = 55
				exph.sum = 5000
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 9, 9: 10,
				}
				return exph
			}(),
			exph2: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = -2
				exph.min = -2500
				exph.sampleCount = 66
				exph.sum = -7000
				exph.scale = 0
				exph.negativeBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 9, 9: 10, 10: 11,
				}
				return exph
			}(),
			expectedExph: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 1100
				exph.min = -2500
				exph.sampleCount = 121
				exph.sum = -2000
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 9, 9: 10,
				}
				exph.negativeBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 9, 9: 10, 10: 11,
				}
				return exph
			}(),
		},
		{
			name: "positive, negative, and zero buckets, non-overlapping and overlapping",
			exph1: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 1100
				exph.min = -2500
				exph.sampleCount = 138
				exph.sum = -2000
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 0, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 0, 8: 9, 9: 10,
				}
				exph.zeroCount = 42
				exph.negativeBuckets = map[int]uint64{
					0: 1, 1: 2, 2: 3, 3: 0, 4: 0, 5: 6, 6: 0, 7: 8, 8: 9, 9: 10, 10: 11,
				}
				return exph
			}(),
			exph2: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 4200
				exph.min = -600
				exph.sampleCount = 118
				exph.sum = 5000
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 0, 1: 2, 2: 3, 3: 0, 4: 5, 5: 6, 6: 0, 7: 8, 8: 9, 9: 10, 10: 11, 11: 0, 12: 13,
				}
				exph.zeroCount = 10
				exph.negativeBuckets = map[int]uint64{
					0: 1, 1: 0, 2: 0, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 0, 9: 10,
				}
				return exph
			}(),
			expectedExph: func() *ExpHistogramDistribution {
				exph := NewExpHistogramDistribution()
				exph.max = 4200
				exph.min = -2500
				exph.sampleCount = 256
				exph.sum = 3000
				exph.scale = 0
				exph.positiveBuckets = map[int]uint64{
					0: 0, 1: 4, 2: 6, 3: 4, 4: 10, 5: 12, 6: 7, 7: 8, 8: 18, 9: 20, 10: 11, 11: 0, 12: 13,
				}
				exph.zeroCount = 52
				exph.negativeBuckets = map[int]uint64{
					0: 2, 1: 2, 2: 3, 3: 4, 4: 5, 5: 12, 6: 7, 7: 16, 8: 9, 9: 20, 10: 11,
				}
				return exph
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exph1 := tt.exph1
			exph2 := tt.exph2
			exph1.AddDistribution(exph2)
			assert.Equal(t, tt.expectedExph, exph1)
		})
	}

	t.Run("different scales", func(t *testing.T) {
		exph1 := NewExpHistogramDistribution()
		exph1.scale = 1
		exph1.max = 48
		exph1.min = 2
		exph1.sampleCount = 21
		exph1.sum = 90
		exph1.positiveBuckets = map[int]uint64{
			0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6,
		}

		exph2 := NewExpHistogramDistribution()
		exph2.scale = 0
		exph2.max = 48
		exph2.min = 2
		exph2.sampleCount = 21
		exph2.sum = 90
		exph2.positiveBuckets = map[int]uint64{
			0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6,
		}

		exph1.AddDistribution(exph2)
		assert.Equal(t, exph1, exph1, "expected exph1 to be unchanged when adding exph2 with different scale")
	})

}

func TestMapToIndexPositiveScale(t *testing.T) {
	tests := []struct {
		name     string
		scale    int
		values   []float64
		expected []int
	}{
		{
			name:     "positive value inside bucket",
			scale:    1,
			values:   []float64{1.3, 1.5, 2.2, 3.9, 4.2, 6.0},
			expected: []int{0, 1, 2, 3, 4, 5},
		},
		{
			// for positive values, histogram buckets use upper-inclusive boundaries
			// this is only reliable on boundaries that are powers of 2
			name:     "positive value is on boundary",
			scale:    1,
			values:   []float64{2.0, 4.0, 8.0},
			expected: []int{1, 3, 5},
		},
		{
			name:     "negative value inside bucket",
			scale:    1,
			values:   []float64{-1.3, -1.5, -2.2, -3.9, -4.2, -6.0},
			expected: []int{0, 1, 2, 3, 4, 5},
		},
		{
			// for negative values, histogram buckets use lower-inclusive boundaries
			// this is only reliable on boundaries that are powers of 2
			name:     "negative value is on boundary",
			scale:    1,
			values:   []float64{-1.0, -2.0, -4.0},
			expected: []int{0, 2, 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, value := range tt.values {
				assert.Equal(t, tt.expected[i], MapToIndex(value, tt.scale), "expected value %f to map to index %d with scale %d", value, tt.expected[i], tt.scale)
			}
		})
	}

}

func TestMapToIndexScale0(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected []int
	}{
		{
			name:     "positive value inside bucket",
			values:   []float64{1.5, 3.0, 6, 12, 18},
			expected: []int{0, 1, 2, 3, 4},
		},
		{
			// for negative values, histogram buckets use lower-inclusive boundaries
			name:     "positive value is on boundary",
			values:   []float64{2, 4, 8, 16.0, 32},
			expected: []int{0, 1, 2, 3, 4},
		},
		{
			name:     "negative value inside bucket",
			values:   []float64{-1.5, -3.0, -6, -12, -18},
			expected: []int{0, 1, 2, 3, 4},
		},
		{
			// for negative values, histogram buckets use lower-inclusive boundaries
			name:     "negative value is on boundary",
			values:   []float64{-2, -4, -8, -16, -32},
			expected: []int{1, 2, 3, 4, 5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, value := range tt.values {
				assert.Equal(t, tt.expected[i], MapToIndexNegativeScale(value, 0), "expected value %f to map to index %d with scale 0", value, tt.expected[i])
			}
		})
	}
}

func TestMapToIndexNegativeScale(t *testing.T) {
	tests := []struct {
		name     string
		scale    int
		values   []float64
		expected []int
	}{
		{
			name:     "positive value inside bucket",
			scale:    -1,
			values:   []float64{1.5, 5.0, 32, 80, 500, 2000},
			expected: []int{0, 1, 2, 3, 4, 5},
		},
		{
			// for positive values, histogram buckets use upper-inclusive boundaries
			name:     "positive value is on boundary",
			scale:    -1,
			values:   []float64{4, 16, 64, 256, 1024},
			expected: []int{0, 1, 2, 3, 4, 5},
		},
		{
			name:     "negative value inside bucket",
			scale:    -1,
			values:   []float64{-1.5, -5.0, -32, -80, -500, -2000},
			expected: []int{0, 1, 2, 3, 4, 5},
		},
		{
			// for negative values, histogram buckets use lower-inclusive boundaries
			name:     "negative value is on boundary",
			scale:    -1,
			values:   []float64{-1, -4, -16, -64, -256, -1024},
			expected: []int{0, 1, 2, 3, 4, 5, 6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, value := range tt.values {
				assert.Equal(t, tt.expected[i], MapToIndexNegativeScale(value, tt.scale), "expected value %f to map to index %d with scale %d", value, tt.expected[i], tt.scale)
			}
		})
	}
}

func TestLowerBoundary(t *testing.T) {
	// scale = 1, base = 2^(1/2) or sqrt(2) = 1.41421
	assert.InDelta(t, 1.41421, LowerBoundary(1, 1), 0.01) // 2^(1/2)
	assert.InDelta(t, 2.0, LowerBoundary(2, 1), 0.01)     // 2^(2/2)
	assert.InDelta(t, 2.82842, LowerBoundary(3, 1), 0.01) // 2^(3/2)
	assert.InDelta(t, 4.0, LowerBoundary(4, 1), 0.01)     // 2^(4/2)
	assert.InDelta(t, 8.0, LowerBoundary(6, 1), 0.01)     // 2^(6/2)
	assert.InDelta(t, 16.0, LowerBoundary(8, 1), 0.01)    // 2^(8/2)

	// scale = 2, base = 2^(1/4) = 1.18921
	assert.InDelta(t, 1.18921, LowerBoundary(1, 2), 0.01) // 2^(1/4)
	assert.InDelta(t, 1.41421, LowerBoundary(2, 2), 0.01) // 2^(2/4)
	assert.InDelta(t, 1.68180, LowerBoundary(3, 2), 0.01) // 2^(3/4)
	assert.InDelta(t, 2.0, LowerBoundary(4, 2), 0.01)     // 2^(4/4)
	assert.InDelta(t, 2.82842, LowerBoundary(6, 2), 0.01) // 2^(6/8)
	assert.InDelta(t, 4.0, LowerBoundary(8, 2), 0.01)     // 2^(8/8)

	// scale = 0, base = 2
	assert.Equal(t, 1.0, LowerBoundary(0, 0)) // 2^0
	assert.Equal(t, 2.0, LowerBoundary(1, 0)) // 2^1
	assert.Equal(t, 4.0, LowerBoundary(2, 0)) // 2^2
	assert.Equal(t, 8.0, LowerBoundary(3, 0)) // 2^3

	assert.Equal(t, 1.0, LowerBoundary(0, -1))  // 4^0
	assert.Equal(t, 4.0, LowerBoundary(1, -1))  // 4^1
	assert.Equal(t, 16.0, LowerBoundary(2, -1)) // 4^2
	assert.Equal(t, 64.0, LowerBoundary(3, -1)) // 4^3

	// scale = -2, base = 2^(2^2) = 2^4 = 16
	assert.Equal(t, 1.0, LowerBoundary(0, -2))    // 16^0
	assert.Equal(t, 16.0, LowerBoundary(1, -2))   // 16^1
	assert.Equal(t, 256.0, LowerBoundary(2, -2))  // 16^2
	assert.Equal(t, 4096.0, LowerBoundary(3, -2)) // 16^3

	assert.Equal(t, 1.0, LowerBoundary(0, -1))  // 4^0
	assert.Equal(t, 4.0, LowerBoundary(1, -1))  // 2^(2^1)^1 = 4^1 = 2^2 = 4^1
	assert.Equal(t, 16.0, LowerBoundary(2, -1)) // (2^2^1)^2 = 4^2 = 2^4 = 4^2
	assert.Equal(t, 64.0, LowerBoundary(3, -1)) // (2^2^1)^3 = 4^3 = 2^6 = 4^3

	// scale = -2, base = 2^(2^2) = 2^4 = 16
	assert.Equal(t, 1.0, LowerBoundary(0, -2))    // (2^(2^2))^0 = 16^0 = 1
	assert.Equal(t, 16.0, LowerBoundary(1, -2))   // (2^(2^2))^1 = 2^4 = 16^1
	assert.Equal(t, 256.0, LowerBoundary(2, -2))  // (2^(2^2))^2 = 2^8 = 16^2
	assert.Equal(t, 4096.0, LowerBoundary(3, -2)) // (2^(2^2))^3 = 2^12 = 16^3
}

func BenchmarkLowerBoundary(b *testing.B) {
	b.Run("positive scale", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			LowerBoundary(10, 1)
		}
	})

	b.Run("scale 0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			LowerBoundary(10, 0)
		}
	})

	b.Run("negative scale", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			LowerBoundary(10, -1)
		}
	})
}

func BenchmarkLowerBoundaryNegativeScale(b *testing.B) {
	b.Run("reference", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			LowerBoundaryNegativeScale(10, -9)
		}
	})
}

func BenchmarkConvertFromOtel(b *testing.B) {

	ts := time.Date(2025, time.March, 31, 22, 6, 30, 0, time.UTC)
	histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
	histogramDP := histogramDPS.AppendEmpty()
	posBucketCounts := make([]uint64, 60)
	for i := range posBucketCounts {
		posBucketCounts[i] = uint64(i % 5) //nolint:gosec
	}
	histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
	histogramDP.SetZeroCount(2)
	negBucketCounts := make([]uint64, 60)
	for i := range negBucketCounts {
		negBucketCounts[i] = uint64(i % 5) //nolint:gosec
	}
	histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
	histogramDP.SetSum(1000)
	histogramDP.SetMin(-9e+17)
	histogramDP.SetMax(9e+17)
	histogramDP.SetCount(uint64(3662))
	histogramDP.SetScale(0)
	histogramDP.Attributes().PutStr("label1", "value1")
	histogramDP.Attributes().PutStr("aws:StorageResolution", "true")
	histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(ts))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h := NewExpHistogramDistribution()
		h.ConvertFromOtel(histogramDP, "count")
	}
}
