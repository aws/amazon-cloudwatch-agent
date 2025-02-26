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

// Configurable flags
const (
	thresholdDays    = 3
	inactiveDays     = 1
	numWorkers       = 15 // Adjust this number based on your needs
	DELETE_BATCH_CAP = 10000
)

var (
	dryRun                       bool
	EXCEPTION_LIST_DO_NOT_DELETE = []string{"lambda"}
)

func init() {
	flag.BoolVar(&dryRun, "dry-run", false, "Enable dry-run mode (no actual deletion)")
	flag.Parse()
}

func main() {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	cfg.RetryMode = aws.RetryModeAdaptive
	if err != nil {
		log.Fatalf("Error loading AWS config: %v", err)
	}

	// Create CloudWatch Logs client
	client := cloudwatchlogs.NewFromConfig(cfg)

	// Compute the cutoff timestamps
	cutoffCreationTime := time.Now().AddDate(0, 0, -thresholdDays).Unix() * 1000
	cutoffInactiveTime := time.Now().AddDate(0, 0, -inactiveDays).Unix() * 1000

	fmt.Printf("🔍 Searching for CloudWatch Log Groups older than %d days AND inactive for %d days. in  %s region\n", thresholdDays, inactiveDays, cfg.Region)

	// Fetch and delete log groups
	deleteOldLogGroups(client, cutoffCreationTime, cutoffInactiveTime)
}

func deleteOldLogGroups(client *cloudwatchlogs.Client, cutoffCreationTime int64, cutoffInactiveTime int64) []string {
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var logGroupsToBeDeleted []string

	// Create a channel to send log groups to workers
	logGroupChan := make(chan *types.LogGroup, 500)
	fmt.Printf("👷 Creating %d of workers\n", numWorkers)
	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for logGroup := range logGroupChan {
				// Skip if creationTime is nil (unlikely)
				if logGroup.CreationTime == nil {
					fmt.Printf("Found faulty log group \n %v", logGroup)
					continue
				}

				logGroupName := *logGroup.LogGroupName
				creationTime := *logGroup.CreationTime

				// Check if log group is older than threshold
				if creationTime < cutoffCreationTime {
					// Check last log event timestamp
					lastLogTime := getLastLogEventTime(client, logGroupName)
					if lastLogTime == 0 {
						return
					}
					if lastLogTime < cutoffInactiveTime {
						fmt.Printf("🚨 Worker: %d| Old & Inactive Log Group: %s (Created: %v, Last Event: %v)\n",
							workerId, logGroupName, time.Unix(creationTime/1000, 0), time.Unix(lastLogTime/1000, 0))
						mutex.Lock()
						logGroupsToBeDeleted = append(logGroupsToBeDeleted, logGroupName)
						mutex.Unlock()
						if dryRun {
							fmt.Printf("🛑 Dry-Run: Would delete %d log groups\n", len(logGroupsToBeDeleted))
							// Dry-run mode - only print
							return
						}
						_, err := client.DeleteLogGroup(context.TODO(), &cloudwatchlogs.DeleteLogGroupInput{
							LogGroupName: logGroup.LogGroupName,
						})
						if err != nil {
							fmt.Printf("❌ Error deleting %s: %v\n", logGroupName, err)
						} else {
							fmt.Printf("✅ Deleted log group: %s\n", logGroupName)
						}
					}
				}
				// fmt.Printf("👷 Worker: %d| No OLD log group found\n", workerId)
			}
		}(i)
	}

	var nextToken *string
	decribeCount := 0
	for {
		// Fetch log groups in pages
		output, err := client.DescribeLogGroups(context.TODO(), &cloudwatchlogs.DescribeLogGroupsInput{
			NextToken: nextToken,
		})
		fmt.Printf("🔍 Described %d times | Found %d log groups now will process them\n", decribeCount, len(output.LogGroups))
		if err != nil {
			log.Fatalf("❌ Failed to retrieve log groups: %v", err)
		}

		// Send log groups to the channel
		for _, logGroup := range output.LogGroups {
			if isLogGroupAnException(*logGroup.LogGroupName) {
				fmt.Printf("⏭️ Skipping Log Group: %s it is in exception list\n", *logGroup.LogGroupName)
				continue
			}
			logGroupChan <- &logGroup
		}
		// Handle pagination
		if output.NextToken == nil {
			break
		}
		mutex.Lock()
		l := len(logGroupsToBeDeleted)
		mutex.Unlock()
		if l > DELETE_BATCH_CAP {
			break
		}
		nextToken = output.NextToken
		decribeCount++
		fmt.Printf("🔍 So far deleted %d\n", l)
	}

	// Close the channel after all log groups have been sent
	close(logGroupChan)

	// Wait for all workers to finish
	wg.Wait()

	// Process the logGroupsToBeDeleted as needed
	fmt.Printf("Log groups to be deleted: %d\n", len(logGroupsToBeDeleted))
	return logGroupsToBeDeleted
}

// getLastLogEventTime fetches the latest log event timestamp for a log group
func getLastLogEventTime(client *cloudwatchlogs.Client, logGroupName string) int64 {
	var latestTimestamp int64
	var nextToken *string

	for {
		// Fetch log streams for the log group
		output, err := client.DescribeLogStreams(context.TODO(), &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logGroupName),
			OrderBy:      types.OrderByLastEventTime,
			Descending:   aws.Bool(true),
			NextToken:    nextToken,
		})
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to retrieve log streams for %s: %v\n", logGroupName, err)
			return 0 // Assume no activity if error occurs
		}

		// Find the latest log event timestamp
		for _, stream := range output.LogStreams {
			if stream.LastEventTimestamp != nil && *stream.LastEventTimestamp > latestTimestamp {
				latestTimestamp = *stream.LastEventTimestamp
			}
		}

		// Handle pagination
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return latestTimestamp
}

func isLogGroupAnException(logGroupName string) bool {
	for _, exception_string := range EXCEPTION_LIST_DO_NOT_DELETE {
		if strings.Contains(logGroupName, exception_string) {
			return true
		}
	}
	return false
}
