package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type DistributionData struct {
	Values []float64 `json:"values"`
	Counts []float64 `json:"counts"`
}

type MetricInfo struct {
	TestCase    string
	MappingType string
	MetricName  string
}

func main() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	cw := cloudwatch.NewFromConfig(cfg)
	testDataDir := "testdata"
	mappingTypes := []string{"cwagent", "exponential", "exponentialcw", "middlepoint", "even"}

	var metrics []MetricInfo

	// Push data to CloudWatch
	for _, mappingType := range mappingTypes {
		dirPath := filepath.Join(testDataDir, mappingType)
		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {

			if err != nil || !strings.HasSuffix(path, ".json") {
				return err
			}

			fileName := strings.TrimSuffix(filepath.Base(path), ".json")
			metricName := fmt.Sprintf("HistogramDistribution_%s", mappingType)
			metrics = append(metrics, MetricInfo{fileName, mappingType, metricName})
			//return nil
			if err := pushDistributionToCloudWatch(ctx, cw, path, mappingType); err != nil {
				fmt.Printf("Error pushing %s: %v\n", path, err)
			}
			return nil
		})

		if err != nil {
			fmt.Printf("Error processing %s: %v\n", mappingType, err)
		}
	}

	// Wait for metrics to be available
	fmt.Println("Waiting for metrics to be available...")
	time.Sleep(30 * time.Second)

	// Query percentiles and display table
	queryPercentilesAndDisplayTable(ctx, cw, metrics)
}

func pushDistributionToCloudWatch(ctx context.Context, cw *cloudwatch.Client, filePath, mappingType string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var dist DistributionData
	if err := json.Unmarshal(data, &dist); err != nil {
		return err
	}

	fileName := strings.TrimSuffix(filepath.Base(filePath), ".json")
	metricName := fmt.Sprintf("HistogramDistribution_%s", mappingType)

	// Convert to CloudWatch distribution format
	values := make([]float64, len(dist.Values))
	counts := make([]float64, len(dist.Counts))
	copy(values, dist.Values)
	copy(counts, dist.Counts)

	_, err = cw.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String("HistogramTesting"),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Dimensions: []types.Dimension{
					{
						Name:  aws.String("TestCase"),
						Value: aws.String(fileName),
					},
					{
						Name:  aws.String("MappingType"),
						Value: aws.String(mappingType),
					},
				},
				Timestamp: aws.Time(time.Now()),
				Values:    values,
				Counts:    counts,
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to push %s: %w", filePath, err)
	}

	fmt.Printf("Pushed %s (%s)\n", fileName, mappingType)
	return nil
}

func queryPercentilesAndDisplayTable(ctx context.Context, cw *cloudwatch.Client, metrics []MetricInfo) {
	percentiles := []string{"10", "25", "50", "75", "90", "99", "99.9"}
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Minute)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TestCase\tMappingType\tP10\tP25\tP50\tP75\tP90\tP99\tP99.9")

	// Group by test case for better organization
	testCases := make(map[string][]MetricInfo)
	for _, metric := range metrics {
		testCases[metric.TestCase] = append(testCases[metric.TestCase], metric)
	}

	// Sort test cases for consistent output
	var sortedTestCases []string
	for testCase := range testCases {
		sortedTestCases = append(sortedTestCases, testCase)
	}
	sort.Strings(sortedTestCases)

	for _, testCase := range sortedTestCases {
		for _, metric := range testCases[testCase] {
			values := make([]string, len(percentiles))
			for i, p := range percentiles {
				stat := fmt.Sprintf("p%s", p)
				resp, err := cw.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
					Namespace:  aws.String("HistogramTesting"),
					MetricName: aws.String(metric.MetricName),
					Dimensions: []types.Dimension{
						{Name: aws.String("TestCase"), Value: aws.String(metric.TestCase)},
						{Name: aws.String("MappingType"), Value: aws.String(metric.MappingType)},
					},
					StartTime:          aws.Time(startTime),
					EndTime:            aws.Time(endTime),
					Period:             aws.Int32(300),
					ExtendedStatistics: []string{stat},
				})
				if err != nil {
					fmt.Printf("Error querying %s: %v\n", metric.MetricName, err)
				}
				if err != nil || len(resp.Datapoints) == 0 {
					values[i] = "N/A"
				} else {
					values[i] = fmt.Sprintf("%.5f", resp.Datapoints[0].ExtendedStatistics[stat])
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				metric.TestCase, metric.MappingType,
				values[0], values[1], values[2], values[3], values[4], values[5], values[6])
		}
	}
	w.Flush()
}
