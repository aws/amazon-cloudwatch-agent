// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package append_dimensions

import (
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
	credsutil "github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

type appendDimensions struct {
}

const SectionKey = "append_dimensions"
const CredsKey = "creds"

var ChildRule = map[string]translator.Rule{}

func (ad *appendDimensions) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})

	//EC2_Metadata_Tags is used to store the metadata tags that user specify.
	var EC2_Metadata_Tags []string
	//EC2_Instance_Tags is used to stroe EC2 Instance Tags associated with this instance.
	var EC2_Instance_Tags = []string{}

	result := map[string]interface{}{}
	temp := map[string]interface{}{}

	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		for _, rule := range ChildRule {
			key, val := rule.ApplyRule(im[SectionKey])
			if key != "" {
				if key == CredsKey {
					temp = translator.MergeTwoUniqueMaps(temp, val.(map[string]interface{}))
				} else if key == "ec2_metadata_tags" {
					EC2_Metadata_Tags = append(EC2_Metadata_Tags, val.(string))
					sort.Strings(EC2_Metadata_Tags)
					temp[key] = EC2_Metadata_Tags
				} else if key == "ec2_instance_tag_keys" {
					EC2_Instance_Tags = append(EC2_Instance_Tags, val.(string))
					sort.Strings(EC2_Instance_Tags)
					temp[key] = EC2_Instance_Tags
				} else {
					temp[key] = val
				}
			}
		}
		result["ec2tagger"] = []interface{}{temp}

		returnKey = "processors"
		returnVal = result
	}
	return
}

func CheckIfExactMatch(input interface{}, desiredKey string, desiredValue string, matchedKey string, matchedValue string) (returnKey string, returnVale string) {
	m := input.(map[string]interface{})
	returnKey = ""
	if v, ok := m[desiredKey]; ok {
		if v.(string) == desiredValue {
			returnKey = matchedKey
			returnVale = matchedValue
		}
	}
	return
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

func init() {
	ad := new(appendDimensions)
	parent.RegisterRule(SectionKey, ad)
	ChildRule["creds"] = credsutil.GetCredsRule(CredsKey)
}
