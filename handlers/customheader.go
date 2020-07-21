// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package handlers

import "github.com/aws/aws-sdk-go/aws/request"

func NewCustomHeaderHandler(name, value string) request.NamedHandler {
	return request.NamedHandler{
		Name: name + "HeaderHandler",
		Fn: func(req *request.Request) {
			req.HTTPRequest.Header.Set(name, value)
		},
	}
}
