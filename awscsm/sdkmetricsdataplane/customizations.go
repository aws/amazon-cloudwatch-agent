// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package sdkmetricsdataplane

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

func init() {
	initClient = defaultInitClientFn
}

func defaultInitClientFn(c *client.Client) {
	c.Handlers.Build.PushBack(overrideAppJSONContentType)
}

func overrideAppJSONContentType(r *request.Request) {
	origCT := r.HTTPRequest.Header.Get("Content-Type")
	fmt.Println("checking content type header", origCT)
	if origCT != "application/json" {
		return
	}

	r.HTTPRequest.Header.Set("Content-Type", "application/x-amz-json-1.1")
}
