// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dockerlabel

import "github.com/aws/private-amazon-cloudwatch-agent-staging/translator"

const (
	SectionKeySDPortLabel = "sd_port_label"
)

type SDPortLabel struct {
}

func (d *SDPortLabel) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeySDPortLabel, "ECS_PROMETHEUS_EXPORTER_PORT", input)
	return
}

func init() {
	RegisterRule(SectionKeySDPortLabel, new(SDPortLabel))
}
