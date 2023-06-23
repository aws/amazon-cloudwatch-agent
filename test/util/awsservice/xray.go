// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsservice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/xray"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
)

const (
	// BatchGetTraces has a max trace ID size of 5.
	batchGetTraceSizes = 5
)

func FilterExpression(annotations map[string]interface{}) string {
	var expression string
	for key, value := range annotations {
		result, err := json.Marshal(value)
		if err != nil {
			continue
		}
		if len(expression) != 0 {
			expression += " AND "
		}
		expression += fmt.Sprintf("annotation.%s = %s", key, result)
	}
	return expression
}

func GetTraceIDs(startTime time.Time, endTime time.Time, filter string) ([]string, error) {
	var traceIDs []string
	input := &xray.GetTraceSummariesInput{StartTime: aws.Time(startTime), EndTime: aws.Time(endTime), FilterExpression: aws.String(filter)}
	for {
		output, err := XrayClient.GetTraceSummaries(context.Background(), input)
		if err != nil {
			return nil, err
		}
		for _, summary := range output.TraceSummaries {
			traceIDs = append(traceIDs, *summary.Id)
		}
		if output.NextToken == nil {
			break
		}
		input.NextToken = output.NextToken
	}
	return traceIDs, nil
}

func GetSegments(traceIDs []string) ([]types.Segment, error) {
	var segments []types.Segment
	traces, err := GetBatchTraces(traceIDs)
	if err != nil {
		return nil, err
	}
	for _, trace := range traces {
		segments = append(segments, trace.Segments...)
	}
	return segments, nil
}

func GetBatchTraces(traceIDs []string) ([]types.Trace, error) {
	var traces []types.Trace
	length := len(traceIDs)
	for i := 0; i < length; i += batchGetTraceSizes {
		j := i + batchGetTraceSizes
		if j > length {
			j = length
		}
		input := &xray.BatchGetTracesInput{TraceIds: traceIDs[i:j]}
		for {
			output, err := XrayClient.BatchGetTraces(context.Background(), input)
			if err != nil {
				return nil, err
			}
			for _, trace := range output.Traces {
				traces = append(traces, trace)
			}
			if output.NextToken == nil {
				break
			}
			input.NextToken = output.NextToken
		}
	}
	return traces, nil
}
