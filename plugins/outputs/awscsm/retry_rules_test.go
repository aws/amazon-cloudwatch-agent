// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm/sdkmetricsdataplane"
)

type mockError struct {
	StatusCodeFn func() int
	RequestIDFn  func() string
}

func (err *mockError) StatusCode() int {
	if err.StatusCodeFn == nil {
		return 200
	}

	return err.StatusCodeFn()
}

func (err *mockError) RequestID() string {
	if err.RequestIDFn == nil {
		return "id"
	}

	return err.RequestIDFn()
}

func (err *mockError) Code() string {
	return "code"
}

func (err *mockError) Message() string {
	return "message"
}

func (err *mockError) Error() string {
	return "error"
}

func (err *mockError) OrigErr() error {
	return nil
}

func TestRetryErrors(t *testing.T) {
	cases := []struct {
		err           error
		expectedRetry bool
	}{
		{
			&mockError{
				StatusCodeFn: func() int {
					return 429
				},
			},
			true,
		},
		{
			&mockError{
				StatusCodeFn: func() int {
					return 502
				},
			},
			true,
		},
		{
			&mockError{
				StatusCodeFn: func() int {
					return 503
				},
			},
			true,
		},
		{
			&mockError{
				StatusCodeFn: func() int {
					return 504
				},
			},
			true,
		},
		{
			&mockError{
				StatusCodeFn: func() int {
					return 301
				},
			},
			false,
		},
	}

	for i, c := range cases {
		a := retryRecordsOnError(c.err)
		e := c.expectedRetry
		if e != a {
			t.Errorf("%d: expected %t, but received %t", i, e, a)
		}
	}
}

func TestRetryRecords(t *testing.T) {
	cases := []struct {
		records         []*sdkmetricsdataplane.SdkMonitoringRecord
		statuses        []*sdkmetricsdataplane.RecordStatus
		expectedRecords []*sdkmetricsdataplane.SdkMonitoringRecord
	}{
		{
			records:         []*sdkmetricsdataplane.SdkMonitoringRecord{},
			statuses:        []*sdkmetricsdataplane.RecordStatus{},
			expectedRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{},
		},
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{},
				{},
				{},
			},
			statuses: []*sdkmetricsdataplane.RecordStatus{
				{},
				{},
				{},
			},
			expectedRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{},
		},
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{},
				{
					Version: aws.String("foo"),
				},
				{},
				{
					Version: aws.String("bar"),
				},
			},
			statuses: []*sdkmetricsdataplane.RecordStatus{
				{},
				{
					Status: aws.String(retryRecordStatus),
				},
				{},
				{
					Status: aws.String(retryRecordStatus),
				},
			},
			expectedRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{
					Version: aws.String("foo"),
				},
				{
					Version: aws.String("bar"),
				},
			},
		},
	}

	for i, c := range cases {
		a := retryRecords(c.records, c.statuses)
		e := c.expectedRecords
		if !reflect.DeepEqual(e, a) {
			t.Errorf("%d: expected %v, but received %v", i, e, a)
		}
	}
}
