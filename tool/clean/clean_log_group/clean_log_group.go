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

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

type cloudwatchlogsClient interface {
	DeleteLogGroup(ctx context.Context, params *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error)
	DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
}

const (
	LogGroupProcessChanSize = 500
)

// Config holds the application configuration
type Config struct {
	creationThreshold time.Duration
	inactiveThreshold time.Duration
	numWorkers        int
	deleteBatchCap    int
	exceptionList     []string
	dryRun            bool
}

// Global configuration
var (
	cfg Config
)

func init() {
	// Set default configuration
	cfg = Config{
		creationThreshold: 3 * clean.KeepDurationOneDay,
		inactiveThreshold: 1 * clean.KeepDurationOneDay,
		numWorkers:        15,
		exceptionList:     []string{"lambda"},
		dryRun:            true,
	}

}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	// Parse command line flags
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "Enable dry-run mode (no actual deletion)")
	flag.Parse()
	// Load AWS configuration
	awsCfg, err := loadAWSConfig(ctx)
	if err != nil {
		log.Fatalf("Error loading AWS config: %v", err)
	}

	// Create CloudWatch Logs client
	client := cloudwatchlogs.NewFromConfig(awsCfg)

	// Compute cutoff times
	cutoffTimes := calculateCutoffTimes()

	log.Printf("🔍 Searching for CloudWatch Log Groups older than %d days AND inactive for %d days in %s region\n",
		cfg.creationThreshold, cfg.inactiveThreshold, awsCfg.Region)

	// Delete old log groups
	deletedGroups := deleteOldLogGroups(ctx, client, cutoffTimes)
	log.Printf("Total log groups deleted: %d", len(deletedGroups))
}

type cutoffTimes struct {
	creation int64
	inactive int64
}

func calculateCutoffTimes() cutoffTimes {
	return cutoffTimes{
		creation: time.Now().Add(cfg.creationThreshold).UnixMilli(),
		inactive: time.Now().Add(cfg.inactiveThreshold).UnixMilli(),
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

func deleteOldLogGroups(ctx context.Context, client cloudwatchlogsClient, times cutoffTimes) []string {
	var (
		wg                      sync.WaitGroup
		deletedLogGroup         []string
		foundLogGroupChan       = make(chan *types.LogGroup, LogGroupProcessChanSize)
		deletedLogGroupNameChan = make(chan string, LogGroupProcessChanSize)
		handlerWg               sync.WaitGroup
	)

	// Start worker pool
	log.Printf("👷 Creating %d workers\n", cfg.numWorkers)
	for i := 0; i < cfg.numWorkers; i++ {
		wg.Add(1)
		w := worker{
			id:                   i,
			wg:                   &wg,
			incomingLogGroupChan: foundLogGroupChan,
			deletedLogGroupChan:  deletedLogGroupNameChan,
			times:                times,
		}
		go w.processLogGroup(ctx, client)
	}

	// Start handler with its own WaitGroup
	handlerWg.Add(1)
	go func() {
		handleDeletedLogGroups(&deletedLogGroup, deletedLogGroupNameChan)
		handlerWg.Done()
	}()

	// Process log groups in batches
	if err := fetchAndProcessLogGroups(ctx, client, foundLogGroupChan); err != nil {
		log.Printf("Error processing log groups: %v", err)
	}

	close(foundLogGroupChan)
	wg.Wait()
	close(deletedLogGroupNameChan)
	handlerWg.Wait()

	return deletedLogGroup
}

func handleDeletedLogGroups(deletedLogGroups *[]string, deletedLogGroupNameChan chan string) {
	for logGroupName := range deletedLogGroupNameChan {
		*deletedLogGroups = append(*deletedLogGroups, logGroupName)
		log.Printf("🔍 Processed %d log groups so far\n", len(*deletedLogGroups))
	}
}

type worker struct {
	id                   int
	wg                   *sync.WaitGroup
	incomingLogGroupChan <-chan *types.LogGroup
	deletedLogGroupChan  chan<- string
	times                cutoffTimes
}

func (w *worker) processLogGroup(ctx context.Context, client cloudwatchlogsClient) {
	defer w.wg.Done()

	for logGroup := range w.incomingLogGroupChan {
		if err := w.handleLogGroup(ctx, client, logGroup); err != nil {
			log.Printf("Worker %d: Error processing log group: %v", w.id, err)
		}
	}
}

func (w *worker) handleLogGroup(ctx context.Context, client cloudwatchlogsClient, logGroup *types.LogGroup) error {
	if logGroup.CreationTime == nil {
		return fmt.Errorf("log group has no creation time: %v", logGroup)
	}

	logGroupName := *logGroup.LogGroupName
	creationTime := *logGroup.CreationTime

	if creationTime >= w.times.creation {
		return nil
	}

	lastLogTime := getLastLogEventTime(ctx, client, logGroupName)
	if lastLogTime == 0 {
		return nil
	}

	if lastLogTime < w.times.inactive {
		log.Printf("🚨 Worker: %d| Old & Inactive Log Group: %s (Created: %v, Last Event: %v)\n",
			w.id, logGroupName, time.Unix(creationTime, 0), time.Unix(lastLogTime, 0))

		w.deletedLogGroupChan <- logGroupName

		if cfg.dryRun {
			log.Printf("🛑 Dry-Run: Would delete log group: %s", logGroupName)
			return nil
		}

		return deleteLogGroup(ctx, client, logGroupName)
	}

	return nil
}

func deleteLogGroup(ctx context.Context, client cloudwatchlogsClient, logGroupName string) error {
	_, err := client.DeleteLogGroup(ctx, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil {
		return fmt.Errorf("deleting log group %s: %w", logGroupName, err)
	}
	log.Printf("✅ Deleted log group: %s", logGroupName)
	return nil
}

func fetchAndProcessLogGroups(ctx context.Context, client cloudwatchlogsClient,
	logGroupChan chan<- *types.LogGroup) error {

	var nextToken *string
	describeCount := 0

	for {
		output, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return fmt.Errorf("describing log groups: %w", err)
		}

		log.Printf("🔍 Described %d times | Found %d log groups\n", describeCount, len(output.LogGroups))

		for _, logGroup := range output.LogGroups {
			if isLogGroupException(*logGroup.LogGroupName) {
				log.Printf("⏭️ Skipping Log Group: %s (in exception list)\n", *logGroup.LogGroupName)
				continue
			}
			logGroupChan <- &logGroup
		}

		if output.NextToken == nil {
			break
		}

		nextToken = output.NextToken
		describeCount++
	}

	return nil
}

func getLastLogEventTime(ctx context.Context, client cloudwatchlogsClient, logGroupName string) int64 {
	var latestTimestamp int64
	var nextToken *string

	for {
		input := &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logGroupName),
			OrderBy:      types.OrderByLastEventTime,
			Descending:   aws.Bool(true),
			NextToken:    nextToken,
		}

		output, err := client.DescribeLogStreams(ctx, input)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to retrieve log streams for %s: %v\n", logGroupName, err)
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
