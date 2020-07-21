// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type CollectionJitter struct {
}

func (c *CollectionJitter) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("collection_jitter", "0s", input)
	return
}

func init() {
	c := new(CollectionJitter)
	RegisterRule("collection_jitter", c)
}
