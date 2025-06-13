// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
)

const maxPersistStateSectionKey = "max_persist_state"

type MaxPersistState struct {
}

func (f *MaxPersistState) ApplyRule(_ any) (string, any) {
	if logs.GlobalLogConfig.Concurrency > 0 {
		return maxPersistStateSectionKey, 2 * logs.GlobalLogConfig.Concurrency
	}

	return "", nil
}

func init() {
	RegisterRule(maxPersistStateSectionKey, new(MaxPersistState))
}
