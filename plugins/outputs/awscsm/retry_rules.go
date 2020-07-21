// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

const (
	statusOK = "OK"
)

func retryRecords(records []*sdkmetricsdataplane.SdkMonitoringRecord, statuses []*sdkmetricsdataplane.RecordStatus) []*sdkmetricsdataplane.SdkMonitoringRecord {
	retryableRecords := []*sdkmetricsdataplane.SdkMonitoringRecord{}

	for i, record := range records {
		if statuses[i].Status != nil && *statuses[i].Status == retryRecordStatus {
			retryableRecords = append(retryableRecords, record)
		}
	}
	return retryableRecords
}

func retryRecordsOnError(err error) bool {
	if aerr, ok := err.(awserr.RequestFailure); ok {
		switch aerr.StatusCode() {
		// retry codes are based determined by APIG's retry policy.
		// https://docs.aws.amazon.com/apigateway/api-reference/handling-errors/
		case 429, 502, 503, 504:
			return true
		}
	}

	return false
}

func failedAmount(statuses []*sdkmetricsdataplane.RecordStatus) int {
	count := 0
	for _, status := range statuses {
		if status.Status != nil && *status.Status != retryRecordStatus && *status.Status != statusOK {
			count++
		}
	}
	return count
}
