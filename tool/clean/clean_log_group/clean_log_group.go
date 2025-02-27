// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// Config holds the application configuration
type Config struct {
	thresholdDays  int
	inactiveDays   int
	numWorkers     int
	deleteBatchCap int
	exceptionList  []string
	dryRun         bool
}

// Logger wraps logging functionality
type Logger struct {
	*log.Logger
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	return &Logger{
		Logger: log.New(log.Writer(), "", log.LstdFlags),
	}
}

// Global configuration
var (
	cfg    Config
	logger *Logger
)

func init() {
	// Initialize logger
	logger = NewLogger()

	// Set default configuration
	cfg = Config{
		thresholdDays:  3,
		inactiveDays:   1,
		numWorkers:     15,
		deleteBatchCap: 10000,
		exceptionList:  []string{"lambda"},
	}

	// Parse command line flags
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "Enable dry-run mode (no actual deletion)")
	flag.Parse()
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Load AWS configuration
	awsCfg, err := loadAWSConfig(ctx)
	if err != nil {
		logger.Fatalf("Error loading AWS config: %v", err)
	}

	// Create CloudWatch Logs client
	client := cloudwatchlogs.NewFromConfig(awsCfg)

	// Compute cutoff times
	cutoffTimes := calculateCutoffTimes()

	logger.Printf("🔍 Searching for CloudWatch Log Groups older than %d days AND inactive for %d days in %s region\n",
		cfg.thresholdDays, cfg.inactiveDays, awsCfg.Region)

	// Delete old log groups
	deletedGroups := deleteOldLogGroups(ctx, client, cutoffTimes)
	logger.Printf("Total log groups deleted: %d", len(deletedGroups))
}

type cutoffTimes struct {
	creation int64
	inactive int64
}

func calculateCutoffTimes() cutoffTimes {
	return cutoffTimes{
		creation: time.Now().AddDate(0, 0, -cfg.thresholdDays).Unix() * 1000,
		inactive: time.Now().AddDate(0, 0, -cfg.inactiveDays).Unix() * 1000,
	}
}

func loadAWSConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}
	cfg.RetryMode = aws.RetryModeAdaptive
	return cfg, nil
}

func deleteOldLogGroups(ctx context.Context, client *cloudwatchlogs.Client, times cutoffTimes) []string {
	var (
		wg                sync.WaitGroup
		mutex             sync.Mutex
		logGroupsToDelete []string
		logGroupChan      = make(chan *types.LogGroup, 500)
	)

	// Start worker pool
	logger.Printf("👷 Creating %d workers\n", cfg.numWorkers)
	for i := 0; i < cfg.numWorkers; i++ {
		wg.Add(1)
		go processLogGroup(ctx, client, logGroupChan, &wg, &mutex, &logGroupsToDelete, times, i)
	}

	// Process log groups in batches
	if err := fetchAndProcessLogGroups(ctx, client, logGroupChan, &logGroupsToDelete, &mutex); err != nil {
		logger.Printf("Error processing log groups: %v", err)
	}

	close(logGroupChan)
	wg.Wait()

	return logGroupsToDelete
}

func processLogGroup(ctx context.Context, client *cloudwatchlogs.Client, logGroupChan <-chan *types.LogGroup,
	wg *sync.WaitGroup, mutex *sync.Mutex, logGroupsToDelete *[]string, times cutoffTimes, workerID int) {
	defer wg.Done()

	for logGroup := range logGroupChan {
		if err := handleLogGroup(ctx, client, logGroup, mutex, logGroupsToDelete, times, workerID); err != nil {
			logger.Printf("Worker %d: Error processing log group: %v", workerID, err)
		}
	}
}

func handleLogGroup(ctx context.Context, client *cloudwatchlogs.Client, logGroup *types.LogGroup,
	mutex *sync.Mutex, logGroupsToDelete *[]string, times cutoffTimes, workerID int) error {

	if logGroup.CreationTime == nil {
		return fmt.Errorf("log group has no creation time: %v", logGroup)
	}

	logGroupName := *logGroup.LogGroupName
	creationTime := *logGroup.CreationTime

	if creationTime >= times.creation {
		return nil
	}

	lastLogTime := getLastLogEventTime(ctx, client, logGroupName)
	if lastLogTime == 0 {
		return nil
	}

	if lastLogTime < times.inactive {
		logger.Printf("🚨 Worker: %d| Old & Inactive Log Group: %s (Created: %v, Last Event: %v)\n",
			workerID, logGroupName, time.Unix(creationTime/1000, 0), time.Unix(lastLogTime/1000, 0))

		mutex.Lock()
		*logGroupsToDelete = append(*logGroupsToDelete, logGroupName)
		mutex.Unlock()

		if cfg.dryRun {
			logger.Printf("🛑 Dry-Run: Would delete log group: %s", logGroupName)
			return nil
		}

		return deleteLogGroup(ctx, client, logGroupName)
	}

	return nil
}

func deleteLogGroup(ctx context.Context, client *cloudwatchlogs.Client, logGroupName string) error {
	_, err := client.DeleteLogGroup(ctx, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil {
		return fmt.Errorf("deleting log group %s: %w", logGroupName, err)
	}
	logger.Printf("✅ Deleted log group: %s", logGroupName)
	return nil
}

func fetchAndProcessLogGroups(ctx context.Context, client *cloudwatchlogs.Client,
	logGroupChan chan<- *types.LogGroup, logGroupsToDelete *[]string, mutex *sync.Mutex) error {

	var nextToken *string
	describeCount := 0

	for {
		output, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return fmt.Errorf("describing log groups: %w", err)
		}

		logger.Printf("🔍 Described %d times | Found %d log groups\n", describeCount, len(output.LogGroups))

		for _, logGroup := range output.LogGroups {
			if isLogGroupException(*logGroup.LogGroupName) {
				logger.Printf("⏭️ Skipping Log Group: %s (in exception list)\n", *logGroup.LogGroupName)
				continue
			}
			logGroupChan <- &logGroup
		}

		if output.NextToken == nil {
			break
		}

		mutex.Lock()
		count := len(*logGroupsToDelete)
		mutex.Unlock()

		if count > cfg.deleteBatchCap {
			break
		}

		nextToken = output.NextToken
		describeCount++
		logger.Printf("🔍 Processed %d log groups so far\n", count)
	}

	return nil
}

func getLastLogEventTime(ctx context.Context, client *cloudwatchlogs.Client, logGroupName string) int64 {
	var latestTimestamp int64
	var nextToken *string

	for {
		output, err := client.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logGroupName),
			OrderBy:      types.OrderByLastEventTime,
			Descending:   aws.Bool(true),
			NextToken:    nextToken,
		})
		if err != nil {
			logger.Printf("⚠️ Warning: Failed to retrieve log streams for %s: %v\n", logGroupName, err)
			return 0
		}

		for _, stream := range output.LogStreams {
			if stream.LastEventTimestamp != nil && *stream.LastEventTimestamp > latestTimestamp {
				latestTimestamp = *stream.LastEventTimestamp
			}
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return latestTimestamp
}

func isLogGroupException(logGroupName string) bool {
	for _, exception := range cfg.exceptionList {
		if strings.Contains(logGroupName, exception) {
			return true
		}
	}
	return false
}
