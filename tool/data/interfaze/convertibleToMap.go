// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package interfaze

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type ConvertibleToMap interface {
	ToMap(context *runtime.Context) (string, map[string]interface{})
}
