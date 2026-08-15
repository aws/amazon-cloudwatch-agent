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
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

type ssmClient interface {
	ListDocuments(ctx context.Context, params *ssm.ListDocumentsInput, optFns ...func(*ssm.Options)) (*ssm.ListDocumentsOutput, error)
	DeleteDocument(ctx context.Context, params *ssm.DeleteDocumentInput, optFns ...func(*ssm.Options)) (*ssm.DeleteDocumentOutput, error)
	DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
	DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
}

const (
	SSMProcessChanSize = 100
)

// Config holds the application configuration
type Config struct {
	creationThreshold     time.Duration
	numWorkers            int
	dryRun                bool
	verbose               bool
	testDocumentPrefixes  []string
	testParameterPrefixes []string
}

// Global configuration
var (
	cfg Config
)

func init() {
	// Set default configuration
	cfg = Config{
		creationThreshold: 1 * clean.KeepDurationOneDay, // Clean documents older than 1 day
		numWorkers:        10,
		dryRun:            true,
		verbose:           false,
		testDocumentPrefixes: []string{
			"Test-AmazonCloudWatch-ManageAgent-", // Used by ssm_document tests
		},
		testParameterPrefixes: []string{
			// Used by ssm_document tests
			"agentConfig1",
			"agentConfig2",
		},
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Parse command line flags
	flag.BoolVar(&cfg.dryRun, "dry-run", true, "Enable dry-run mode (no actual deletion)")
	flag.BoolVar(&cfg.verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	// Load AWS configuration
	awsCfg, err := loadAWSConfig(ctx)
	if err != nil {
		log.Fatalf("Error loading AWS config: %v", err)
	}

	// Create SSM client
	client := ssm.NewFromConfig(awsCfg)

	// Compute cutoff time (resources older than this will be cleaned)
	cutoffTime := time.Now().Add(cfg.creationThreshold) // creationThreshold is negative, so this gives us a past time

	log.Printf("🔍 Searching for test SSM documents and parameters older than %v (cutoff: %v) in %s region\n",
		-cfg.creationThreshold, cutoffTime.Format("2006-01-02 15:04:05 UTC"), awsCfg.Region)

	// Clean old test documents
	deletedDocs := cleanOldTestDocuments(ctx, client, cutoffTime)
	log.Printf("📄 Total test documents processed: %d", len(deletedDocs))

	// Clean old test parameters
	deletedParams := cleanOldTestParameters(ctx, client, cutoffTime)
	log.Printf("⚙️  Total test parameters processed: %d", len(deletedParams))

	// Summary
	if len(deletedDocs) > 0 || len(deletedParams) > 0 {
		log.Printf("✅ Cleanup completed: %d documents, %d parameters", len(deletedDocs), len(deletedParams))
	} else {
		log.Printf("✅ No test resources found to clean up")
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

func cleanOldTestDocuments(ctx context.Context, client ssmClient, cutoffTime time.Time) []string {
	var (
		wg                  sync.WaitGroup
		deletedDocuments    []string
		foundDocumentChan   = make(chan *types.DocumentIdentifier, SSMProcessChanSize)
		deletedDocumentChan = make(chan string, SSMProcessChanSize)
		handlerWg           sync.WaitGroup
	)

	// Start worker pool
	log.Printf("👷 Creating %d workers for document cleanup\n", cfg.numWorkers)
	for i := 0; i < cfg.numWorkers; i++ {
		wg.Add(1)
		w := documentWorker{
			id:                   i,
			wg:                   &wg,
			incomingDocumentChan: foundDocumentChan,
			deletedDocumentChan:  deletedDocumentChan,
			cutoffTime:           cutoffTime,
		}
		go w.processDocument(ctx, client)
	}

	// Start handler
	handlerWg.Add(1)
	go func() {
		handleDeletedDocuments(&deletedDocuments, deletedDocumentChan)
		handlerWg.Done()
	}()

	// Fetch and process documents
	if err := fetchAndProcessDocuments(ctx, client, foundDocumentChan); err != nil {
		log.Printf("Error processing documents: %v", err)
	}

	close(foundDocumentChan)
	wg.Wait()
	close(deletedDocumentChan)
	handlerWg.Wait()

	return deletedDocuments
}

func cleanOldTestParameters(ctx context.Context, client ssmClient, cutoffTime time.Time) []string {
	var (
		wg                   sync.WaitGroup
		deletedParameters    []string
		foundParameterChan   = make(chan *types.ParameterMetadata, SSMProcessChanSize)
		deletedParameterChan = make(chan string, SSMProcessChanSize)
		handlerWg            sync.WaitGroup
	)

	// Start worker pool
	log.Printf("👷 Creating %d workers for parameter cleanup\n", cfg.numWorkers)
	for i := 0; i < cfg.numWorkers; i++ {
		wg.Add(1)
		w := parameterWorker{
			id:                    i,
			wg:                    &wg,
			incomingParameterChan: foundParameterChan,
			deletedParameterChan:  deletedParameterChan,
			cutoffTime:            cutoffTime,
		}
		go w.processParameter(ctx, client)
	}

	// Start handler
	handlerWg.Add(1)
	go func() {
		handleDeletedParameters(&deletedParameters, deletedParameterChan)
		handlerWg.Done()
	}()

	// Fetch and process parameters
	if err := fetchAndProcessParameters(ctx, client, foundParameterChan); err != nil {
		log.Printf("Error processing parameters: %v", err)
	}

	close(foundParameterChan)
	wg.Wait()
	close(deletedParameterChan)
	handlerWg.Wait()

	return deletedParameters
}

type documentWorker struct {
	id                   int
	wg                   *sync.WaitGroup
	incomingDocumentChan <-chan *types.DocumentIdentifier
	deletedDocumentChan  chan<- string
	cutoffTime           time.Time
}

func (w *documentWorker) processDocument(ctx context.Context, client ssmClient) {
	defer w.wg.Done()

	for document := range w.incomingDocumentChan {
		if err := w.handleDocument(ctx, client, document); err != nil {
			log.Printf("Worker %d: Error processing document: %v", w.id, err)
		}
	}
}

func (w *documentWorker) handleDocument(ctx context.Context, client ssmClient, document *types.DocumentIdentifier) error {
	if document.CreatedDate == nil || document.Name == nil {
		return fmt.Errorf("document has missing required fields: %v", document)
	}

	documentName := *document.Name
	createdDate := *document.CreatedDate

	// Check if document is old enough and matches test patterns
	if createdDate.After(w.cutoffTime) {
		// Document is too new, skip it
		return nil
	}

	if !isTestDocument(documentName) {
		return nil
	}

	log.Printf("🚨 Worker: %d| Old Test Document: %s (Created: %v)\n",
		w.id, documentName, createdDate)

	w.deletedDocumentChan <- documentName

	if cfg.dryRun {
		log.Printf("🛑 Dry-Run: Would delete document: %s", documentName)
		return nil
	}

	return deleteDocument(ctx, client, documentName)
}

type parameterWorker struct {
	id                    int
	wg                    *sync.WaitGroup
	incomingParameterChan <-chan *types.ParameterMetadata
	deletedParameterChan  chan<- string
	cutoffTime            time.Time
}

func (w *parameterWorker) processParameter(ctx context.Context, client ssmClient) {
	defer w.wg.Done()

	for parameter := range w.incomingParameterChan {
		if err := w.handleParameter(ctx, client, parameter); err != nil {
			log.Printf("Worker %d: Error processing parameter: %v", w.id, err)
		}
	}
}

func (w *parameterWorker) handleParameter(ctx context.Context, client ssmClient, parameter *types.ParameterMetadata) error {
	if parameter.LastModifiedDate == nil || parameter.Name == nil {
		return fmt.Errorf("parameter has missing required fields: %v", parameter)
	}

	parameterName := *parameter.Name
	lastModified := *parameter.LastModifiedDate

	// Check if parameter is old enough and matches test patterns
	if lastModified.After(w.cutoffTime) {
		// Parameter is too new, skip it
		return nil
	}

	if !isTestParameter(parameterName) {
		return nil
	}

	log.Printf("🚨 Worker: %d| Old Test Parameter: %s (Last Modified: %v)\n",
		w.id, parameterName, lastModified)

	w.deletedParameterChan <- parameterName

	if cfg.dryRun {
		log.Printf("🛑 Dry-Run: Would delete parameter: %s", parameterName)
		return nil
	}

	return deleteParameter(ctx, client, parameterName)
}

func handleDeletedDocuments(deletedDocuments *[]string, deletedDocumentChan chan string) {
	for documentName := range deletedDocumentChan {
		*deletedDocuments = append(*deletedDocuments, documentName)
		// Only log every 10 processed items to reduce noise, or if verbose mode is enabled
		if cfg.verbose || len(*deletedDocuments)%10 == 0 {
			log.Printf("🔍 Processed %d documents so far\n", len(*deletedDocuments))
		}
	}
}

func handleDeletedParameters(deletedParameters *[]string, deletedParameterChan chan string) {
	for parameterName := range deletedParameterChan {
		*deletedParameters = append(*deletedParameters, parameterName)
		// Log each parameter since there should be fewer of them
		log.Printf("🔍 Processed %d parameters so far\n", len(*deletedParameters))
	}
}

func deleteDocument(ctx context.Context, client ssmClient, documentName string) error {
	_, err := client.DeleteDocument(ctx, &ssm.DeleteDocumentInput{
		Name: aws.String(documentName),
	})
	if err != nil {
		return fmt.Errorf("deleting document %s: %w", documentName, err)
	}
	log.Printf("✅ Deleted document: %s", documentName)
	return nil
}

func deleteParameter(ctx context.Context, client ssmClient, parameterName string) error {
	_, err := client.DeleteParameter(ctx, &ssm.DeleteParameterInput{
		Name: aws.String(parameterName),
	})
	if err != nil {
		return fmt.Errorf("deleting parameter %s: %w", parameterName, err)
	}
	log.Printf("✅ Deleted parameter: %s", parameterName)
	return nil
}

func fetchAndProcessDocuments(ctx context.Context, client ssmClient, documentChan chan<- *types.DocumentIdentifier) error {
	var nextToken *string
	describeCount := 0

	for {
		output, err := client.ListDocuments(ctx, &ssm.ListDocumentsInput{
			NextToken:  nextToken,
			MaxResults: aws.Int32(50), // Process in batches
		})
		if err != nil {
			return fmt.Errorf("listing documents: %w", err)
		}

		// Only log every 10th batch to reduce noise, or if verbose mode is enabled
		if cfg.verbose || describeCount%10 == 0 || output.NextToken == nil {
			log.Printf("🔍 Scanned %d batches | Found %d documents in current batch\n", describeCount+1, len(output.DocumentIdentifiers))
		}

		for _, document := range output.DocumentIdentifiers {
			documentChan <- &document
		}

		if output.NextToken == nil {
			break
		}

		nextToken = output.NextToken
		describeCount++
	}

	return nil
}

func fetchAndProcessParameters(ctx context.Context, client ssmClient, parameterChan chan<- *types.ParameterMetadata) error {
	var nextToken *string
	describeCount := 0

	for {
		output, err := client.DescribeParameters(ctx, &ssm.DescribeParametersInput{
			NextToken:  nextToken,
			MaxResults: aws.Int32(50), // Process in batches
		})
		if err != nil {
			return fmt.Errorf("describing parameters: %w", err)
		}

		// Only log every 10th batch to reduce noise, or if verbose mode is enabled
		if cfg.verbose || describeCount%10 == 0 || output.NextToken == nil {
			log.Printf("🔍 Scanned %d batches | Found %d parameters in current batch\n", describeCount+1, len(output.Parameters))
		}

		for _, parameter := range output.Parameters {
			parameterChan <- &parameter
		}

		if output.NextToken == nil {
			break
		}

		nextToken = output.NextToken
		describeCount++
	}

	return nil
}

func isTestDocument(documentName string) bool {
	for _, prefix := range cfg.testDocumentPrefixes {
		if strings.HasPrefix(documentName, prefix) {
			return true
		}
	}
	return false
}

func isTestParameter(parameterName string) bool {
	for _, exactName := range cfg.testParameterPrefixes {
		if parameterName == exactName {
			return true
		}
	}
	return false
}
