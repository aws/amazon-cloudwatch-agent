// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import "github.com/aws/amazon-cloudwatch-agent/translator"

const (
	SectionKeySDResultFile = "sd_result_file"

	defaultPath = "/tmp/cwagent_ecs_auto_sd.yaml"
)

type SDResultFile struct {
}

func (d *SDResultFile) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeySDResultFile, defaultPath, input)
	return
}

func init() {
	RegisterRule(SectionKeySDResultFile, new(SDResultFile))
}
