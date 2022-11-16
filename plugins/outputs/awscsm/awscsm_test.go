// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm/sdkmetricsdataplane"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/awscsm/metametrics"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/awscsm/providers"
)

type mockDataplane struct {
	*sdkmetricsdataplane.SDKMetricsDataplane
	called  int
	outputs []*sdkmetricsdataplane.PutRecordsOutput
	errors  []error
}

func (service *mockDataplane) PutRecords(input *sdkmetricsdataplane.PutRecordsInput) (*sdkmetricsdataplane.PutRecordsOutput, error) {
	output := service.outputs[service.called]
	err := service.errors[service.called]
	service.called++

	return output, err
}

func TestPublish(t *testing.T) {
	cases := []struct {
		records     []*sdkmetricsdataplane.SdkMonitoringRecord
		recordLimit int
		outputs     []*sdkmetricsdataplane.PutRecordsOutput
		errors      []error

		expectedRemainingRecords []*sdkmetricsdataplane.SdkMonitoringRecord
		expectedCallAmount       int
		expectedSendRecordLimit  int
	}{
		// Case 0: generic success, no leftovers
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{
					Version: aws.String("0"),
				},
				{
					Version: aws.String("1"),
				},
				{
					Version: aws.String("2"),
				},
				{
					Version: aws.String("3"),
				},
			},
			recordLimit: 1,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				{
					Statuses: []*sdkmetricsdataplane.RecordStatus{
						{},
						{},
						{},
						{},
					},
				},
			},
			errors: []error{
				nil,
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{},
			expectedCallAmount:       1,
			expectedSendRecordLimit:  1,
		},

		// Case 1: generic success with leftovers
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{
					Version: aws.String("0"),
				},
				{
					Version: aws.String("1"),
				},
				{
					Version: aws.String("2"),
				},
				{
					Version: aws.String("3"),
				},
				{
					Version: aws.String("4"),
				},
				{
					Version: aws.String("5"),
				},
			},
			recordLimit: 1,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				{
					Statuses: []*sdkmetricsdataplane.RecordStatus{
						{},
						{},
						{},
						{},
						{},
					},
				},
			},
			errors: []error{
				nil,
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{
					Version: aws.String("0"),
				},
			},
			expectedCallAmount:      1,
			expectedSendRecordLimit: 6,
		},

		// Case 2: nothing to do
		{
			records:                  []*sdkmetricsdataplane.SdkMonitoringRecord{},
			recordLimit:              1,
			outputs:                  []*sdkmetricsdataplane.PutRecordsOutput{},
			errors:                   []error{},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{},
			expectedCallAmount:       0,
			expectedSendRecordLimit:  1,
		},

		// Case 3: retryable failure, nothing leftover
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
			},
			recordLimit: 3,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				nil,
			},
			errors: []error{
				awserr.NewRequestFailure(awserr.New("TestException", "testing", nil), 503, "TestId"),
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
			},
			expectedCallAmount:      1,
			expectedSendRecordLimit: 1,
		},

		// Case 4: retryable failure with leftovers
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
				{Version: aws.String("4")},
				{Version: aws.String("5")},
			},
			recordLimit: 3,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				nil,
			},
			errors: []error{
				awserr.NewRequestFailure(awserr.New("TestException", "testing", nil), 503, "TestId"),
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
				{Version: aws.String("4")},
				{Version: aws.String("5")},
			},
			expectedCallAmount:      1,
			expectedSendRecordLimit: 1,
		},

		// Case 5: non-retryable failure, nothing leftover
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
			},
			recordLimit: 3,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				nil,
			},
			errors: []error{
				awserr.NewRequestFailure(awserr.New("TestException", "testing", nil), 400, "TestId"),
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{},
			expectedCallAmount:       1,
			expectedSendRecordLimit:  3,
		},

		// Case 6: non-retryable failure with leftovers
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
				{Version: aws.String("4")},
				{Version: aws.String("5")},
			},
			recordLimit: 3,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				nil,
			},
			errors: []error{
				awserr.NewRequestFailure(awserr.New("TestException", "testing", nil), 400, "TestId"),
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
			},
			expectedCallAmount:      1,
			expectedSendRecordLimit: 1,
		},

		// Case 7: early error interrupts otherwise-longer publish
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
				{Version: aws.String("4")},
				{Version: aws.String("5")},
			},
			recordLimit: 10,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				nil,
			},
			errors: []error{
				awserr.NewRequestFailure(awserr.New("TestException", "testing", nil), 400, "TestId"),
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
			},
			expectedCallAmount:      1,
			expectedSendRecordLimit: 5,
		},

		// Case 8: Multiple puts in one publish, nothing leftover
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
				{Version: aws.String("4")},
				{Version: aws.String("5")},
			},
			recordLimit: 10,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				{
					Statuses: []*sdkmetricsdataplane.RecordStatus{
						{},
						{},
						{},
						{},
						{},
					},
				},
				{
					Statuses: []*sdkmetricsdataplane.RecordStatus{
						{},
					},
				},
			},
			errors: []error{
				nil,
				nil,
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{},
			expectedCallAmount:       2,
			expectedSendRecordLimit:  10,
		},

		// Case 9: Multiple puts in one publish with leftovers
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
				{Version: aws.String("4")},
				{Version: aws.String("5")},
				{Version: aws.String("6")},
				{Version: aws.String("7")},
				{Version: aws.String("8")},
				{Version: aws.String("9")},
				{Version: aws.String("10")},
				{Version: aws.String("11")},
			},
			recordLimit: 10,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				{
					Statuses: []*sdkmetricsdataplane.RecordStatus{
						{},
						{},
						{},
						{},
						{},
					},
				},
				{
					Statuses: []*sdkmetricsdataplane.RecordStatus{
						{},
						{},
						{},
						{},
						{},
					},
				},
			},
			errors: []error{
				nil,
				nil,
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
			},
			expectedCallAmount:      2,
			expectedSendRecordLimit: 15,
		},

		// Case 10: success with retryable failed records with leftovers
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
				{Version: aws.String("4")},
				{Version: aws.String("5")},
			},
			recordLimit: 5,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				{
					Statuses: []*sdkmetricsdataplane.RecordStatus{
						{Status: aws.String(retryRecordStatus)}, // 5
						{},                                      // 4
						{Status: aws.String(retryRecordStatus)}, // 3
						{},                                      // 2
						{},                                      // 1
					},
				},
			},
			errors: []error{
				nil,
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")}, // wasn't sent
				{Version: aws.String("3")}, // failed
				{Version: aws.String("5")}, // failed
			},
			expectedCallAmount:      1,
			expectedSendRecordLimit: 10,
		},

		// Case 11: success with non-retryable failed records with leftovers
		{
			records: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("1")},
				{Version: aws.String("2")},
				{Version: aws.String("3")},
				{Version: aws.String("4")},
				{Version: aws.String("5")},
			},
			recordLimit: 5,
			outputs: []*sdkmetricsdataplane.PutRecordsOutput{
				{
					Statuses: []*sdkmetricsdataplane.RecordStatus{
						{Status: aws.String("NON-RETRYABLE ERROR")},
						{},
						{Status: aws.String(retryRecordStatus)},
						{},
						{},
					},
				},
			},
			errors: []error{
				nil,
			},
			expectedRemainingRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
				{Version: aws.String("0")},
				{Version: aws.String("3")},
			},
			expectedCallAmount:      1,
			expectedSendRecordLimit: 10,
		},
	}

	svc := sdkmetricsdataplane.New(session.New())
	for i, c := range cases {
		mockService := &mockDataplane{
			svc,
			0,
			c.outputs,
			c.errors,
		}

		csm := &CSM{
			sendRecordLimit: c.recordLimit,
			logger:          noopLogger{},
			dataplaneClient: mockService,
		}

		ring := newRecordRing(1000000)
		for _, r := range c.records {
			ring.pushFront(r)
		}

		csm.publish(&ring)
		remainingRecords := ring.toSlice()

		if e, a := c.expectedRemainingRecords, remainingRecords; !reflect.DeepEqual(e, a) {
			t.Errorf("Case %d: remaining records expected %v, but received %v", i, e, a)
		}

		if e, a := c.expectedCallAmount, mockService.called; e != a {
			t.Errorf("Case %d: expected call amount %v, but received %v", i, e, a)
		}

		if e, a := c.expectedSendRecordLimit, csm.sendRecordLimit; e != a {
			t.Errorf("Case %d: expected send record limit %v, but received %v", i, e, a)
		}
	}
}

func TestCompression(t *testing.T) {
	cases := []struct {
		samples                 []map[string]interface{}
		expectedBase64          string
		expectedChecksum        int64
		expectedUncompressedLen int64
		expectedError           error
	}{
		{
			samples:                 []map[string]interface{}{},
			expectedBase64:          "H4sIAAAAAAAA/4qOBQQAAP//KbtMDQIAAAA=",
			expectedChecksum:        1984806262,
			expectedUncompressedLen: 2,
		},
		{
			samples: []map[string]interface{}{
				{
					"foo": "bar",
					"baz": 1,
				},
			},
			expectedBase64:          "H4sIAAAAAAAA/4quVkpKrFKyMtRRSsvPV7JSSkosUqqNBQQAAP//OHGadBcAAAA=",
			expectedChecksum:        751326492,
			expectedUncompressedLen: 23,
		},
	}

	for _, c := range cases {
		b64, checksum, l, err := compressSamples(c.samples)

		if e, a := c.expectedError, err; e != a {
			t.Errorf("expected %v, but received %v", e, a)
		}

		if e, a := c.expectedBase64, b64; e != a {
			t.Errorf("expected %s, but received %s", e, a)
		}

		if e, a := c.expectedChecksum, checksum; e != a {
			t.Errorf("expected %d, but received %d", e, a)
		}

		if e, a := c.expectedUncompressedLen, l; e != a {
			t.Errorf("expected %d, but received %d", e, a)
		}
	}
}

func init() {
	mock := &providers.MockConfigProvider{}
	metametrics.MetricListener = metametrics.NewListenerAndStart(mock, 10, 1*time.Second)
	providers.Config = mock
}
