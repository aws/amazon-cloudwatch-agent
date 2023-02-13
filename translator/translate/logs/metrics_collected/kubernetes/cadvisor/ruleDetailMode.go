// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cadvisor

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const (
	// It could be "basic", "detail"
	SectionKeyMode = "mode"
	defaultMode    = "detail"
)

type DetailMode struct {
}

func (d *DetailMode) ApplyRule(input interface{}) (string, interface{}) {
	return translator.DefaultCase(SectionKeyMode, defaultMode, input)
}

func init() {
	RegisterRule(SectionKeyMode, new(DetailMode))
}
