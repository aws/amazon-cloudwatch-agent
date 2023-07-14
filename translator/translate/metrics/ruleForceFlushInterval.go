// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type ForceFlushInterval struct {
}

func (f *ForceFlushInterval) ApplyRule(input interface{}) (string, interface{}) {
	key, val := translator.DefaultTimeIntervalCase("force_flush_interval", float64(60), input)
	return "outputs", map[string]interface{}{key: val}
}

func init() {
	RegisterRule("forceFlushInterval", new(ForceFlushInterval))
}
