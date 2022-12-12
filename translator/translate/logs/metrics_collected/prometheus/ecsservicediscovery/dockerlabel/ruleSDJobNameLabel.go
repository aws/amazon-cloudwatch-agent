// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dockerlabel

import "github.com/aws/private-amazon-cloudwatch-agent-staging/translator"

const (
	SectionKeySDJobNameLabel = "sd_job_name_label"
)

type SDJobNameLabel struct {
}

func (d *SDJobNameLabel) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeySDJobNameLabel, "job", input)
	return
}

func init() {
	RegisterRule(SectionKeySDJobNameLabel, new(SDJobNameLabel))
}
