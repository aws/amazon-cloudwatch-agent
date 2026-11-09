// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsprovisionerextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscloudwatchlogsprovisionerextension"

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

type defaultCWLogsClient struct {
	svc *cloudwatchlogs.Client
}

func newDefaultCWLogsClient(ctx context.Context, region string, timeout time.Duration) (cwLogsClient, error) {
	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithHTTPClient(&http.Client{Timeout: timeout}),
	)
	if err != nil {
		return nil, err
	}
	return &defaultCWLogsClient{svc: cloudwatchlogs.NewFromConfig(cfg)}, nil
}

func (c *defaultCWLogsClient) CreateLogGroup(ctx context.Context, logGroupName string) error {
	_, err := c.svc.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil && !isAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *defaultCWLogsClient) CreateLogStream(ctx context.Context, logGroupName, logStreamName string) error {
	_, err := c.svc.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: aws.String(logStreamName),
	})
	if err != nil && !isAlreadyExists(err) {
		return err
	}
	return nil
}

func isAlreadyExists(err error) bool {
	var alreadyExists *types.ResourceAlreadyExistsException
	return errors.As(err, &alreadyExists)
}

func isNotFound(err error) bool {
	var notFound *types.ResourceNotFoundException
	return errors.As(err, &notFound)
}
