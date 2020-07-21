// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

type FixedTailConfig struct {
}

func (f *FixedTailConfig) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	rm := map[string]interface{}{
		"destination": "cloudwatchlogs",
	}
	return "fixedTailConfig", rm
}

func init() {
	RegisterRule("fixedTailConfig", new(FixedTailConfig))
}
