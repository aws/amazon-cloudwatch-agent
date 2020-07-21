// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cadvisor

const (
	SectionKeyMode = "mode"
	defaultMode    = "detail"
)

type DetailMode struct {
}

func (d *DetailMode) ApplyRule(input interface{}) (string, interface{}) {
	return SectionKeyMode, defaultMode
}

func init() {
	RegisterRule(SectionKeyMode, new(DetailMode))
}
