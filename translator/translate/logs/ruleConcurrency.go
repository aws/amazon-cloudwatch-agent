// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import "github.com/aws/amazon-cloudwatch-agent/translator"

const ConcurrencySectionKey = "concurrency"

type Concurrency struct {
}

func (c *Concurrency) ApplyRule(input any) (string, any) {
	result := map[string]interface{}{}
	_, val := translator.DefaultCase(ConcurrencySectionKey, float64(0), input)
	if v, ok := val.(float64); ok && v > 0 {
		result[ConcurrencySectionKey] = int(v)
	}
	return Output_Cloudwatch_Logs, result
}

func init() {
	RegisterRule(ConcurrencySectionKey, new(Concurrency))
}
