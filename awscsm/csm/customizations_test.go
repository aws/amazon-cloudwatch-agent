// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build go1.8
// +build go1.8

package csm_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/awscsm/csm"
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
				svc := csm.New(unit.Session)
				req, _ := svc.GetPublishingSchemaRequest(&csm.GetPublishingSchemaInput{
					SchemaVersion: aws.String("abc"),
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
