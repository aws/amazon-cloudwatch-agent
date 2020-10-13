package taskdefinition

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	SectionKeySDTaskDefinitionArnPattern = "sd_task_definition_arn_pattern"
)

type SDTaskDefinitionArnPattern struct {
}

// Mandatory Key
func (d *SDTaskDefinitionArnPattern) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	if val, ok := im[SectionKeySDTaskDefinitionArnPattern]; !ok {
		returnKey = ""
		returnVal = ""
		translator.AddErrorMessages(GetCurPath()+SectionKeySDTaskDefinitionArnPattern, "sd_task_definition_arn_pattern is not defined.")
	} else {
		returnKey = SectionKeySDTaskDefinitionArnPattern
		returnVal = val
	}
	return
}

func init() {
	RegisterRule(SectionKeySDTaskDefinitionArnPattern, new(SDTaskDefinitionArnPattern))
}
