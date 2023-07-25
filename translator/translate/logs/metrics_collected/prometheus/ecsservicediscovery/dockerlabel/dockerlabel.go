// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dockerlabel

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SubSectionKey = "docker_label"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SubSectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type DockerLabel struct {
}

func (e *DockerLabel) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]interface{}{}
	returnKey = SubSectionKey

	if _, ok := im[SubSectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		for _, rule := range ChildRule {
			key, val := rule.ApplyRule(im[SubSectionKey])
			if key != "" {
				result[key] = val
			}
		}
		returnKey = SubSectionKey
		returnVal = result
	}
	return
}

func init() {
	e := new(DockerLabel)
	parent.RegisterRule(SubSectionKey, e)
}
