// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func GetTargetPlatform() string {
	return context.CurrentContext().Os()
}

func SetTargetPlatform(targetPlatform string) {
	context.CurrentContext().SetOs(targetPlatform)
}
