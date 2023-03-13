package integration

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
)

func validateConfig(config Config) (string, string) {
	s3Bucket, ok := config["s3Bucket"].(string)
	if !ok {
		log.Fatal("Error: s3Bucket was not provided in config.json")
	}

	cwaGithubSha, ok := config["cwaGithubSha"].(string)
	if !ok {
		log.Fatal("Error: cwaGithubSha was not provided in config.json")
	}
	return s3Bucket, cwaGithubSha
}

func buildKey(cwaGithubSha string) string {
	return fmt.Sprintf("integration-test/binary/%v", cwaGithubSha)
}

func CheckBinaryExists(config Config) bool {
	s3Bucket, cwaGithubSha := validateConfig(config)

	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	// Create an Amazon S3 service client
	client := s3.NewFromConfig(cfg)

	// Get the first page of results for ListObjectsV2 for a bucket
	prefix := buildKey(cwaGithubSha)
	output, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(s3Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		log.Fatal(err)
	}

	exists := len(output.Contents) > 0
	if !exists {
		log.Fatalf("Error: a binary with the following SHA has not been uploaded to s3 yet: %v", cwaGithubSha)
	}
}
