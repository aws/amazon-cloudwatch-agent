// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

type Region struct {
}

const (
	RegionKey  = "region"
	RegionType = "region_type"
)

// This region will be provided to the corresponding input and output plugins
// This should be applied before interpreting other component.
func (r *Region) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	ctx := context.CurrentContext()
	_, inputRegion := translator.DefaultCase(RegionKey, "", input)
	if inputRegion != "" {
		Global_Config.Region = inputRegion.(string)
		Global_Config.RegionType = config.RegionTypeAgentConfigJson
		return
	}
	region, regionType := util.DetectRegion(ctx.Mode(), ctx.Credentials())

	if region == "" {
		translator.AddErrorMessages(GetCurPath()+"ruleRegion/", fmt.Sprintf("Region info is missing for mode: %s",
			ctx.Mode()))
	}

	Global_Config.Region = region
	Global_Config.RegionType = regionType
	return
}

func init() {
	r := new(Region)
	RegisterRule(RegionKey, r)
}
