// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build go1.8
// +build go1.8

package sdkmetricsdataplane_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting/unit"
)

func TestOverrideAppJSONContentType(t *testing.T) {
	cases := map[string]struct {
		DoOpFn   func() *request.Request
		ExpectCT string
	}{
		"Override": {
			DoOpFn: func() *request.Request {
				svc := sdkmetricsdataplane.New(unit.Session)
				req, _ := svc.PutRecordsRequest(&sdkmetricsdataplane.PutRecordsInput{
					Environment: &sdkmetricsdataplane.HostEnvironment{
						Os: aws.String("os"),
					},
					SdkRecords: []*sdkmetricsdataplane.SdkMonitoringRecord{
						{
							AggregationKey: &sdkmetricsdataplane.SdkAggregationKey{
								Timestamp: aws.Time(time.Now()),
							},
							Version: aws.String("version"),
							Id:      aws.String("id"),
						},
					},
				})

				return req
			},
			ExpectCT: "application/x-amz-json-1.1",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			req := c.DoOpFn()

			if err := req.Sign(); err != nil {
				t.Errorf("expect no error, got %v", err)
			}

			var body bytes.Buffer
			io.Copy(&body, req.Body)
			if e, a := c.ExpectCT, req.HTTPRequest.Header.Get("Content-Type"); e != a {
				t.Errorf("expect %v content type, got %v, payload: %s", e, a, body.String())
			}
		})
	}
}
