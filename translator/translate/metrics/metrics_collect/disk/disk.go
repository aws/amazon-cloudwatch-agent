package disk

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

const SectionKey_Disk_Linux = "disk"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Disk_Linux + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type Disk struct {
}

func (d *Disk) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Disk_Linux]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		/*
		  In JSON config file, it represent as "disk" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are some config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Disk_Linux], ChildRule, result)

		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_Disk_Linux], SectionKey_Disk_Linux, GetCurPath(), result)
		if hasValidMetric {
			resArray = append(resArray, result)
			returnKey = SectionKey_Disk_Linux
			returnVal = resArray
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	d := new(Disk)
	parent.RegisterLinuxRule(SectionKey_Disk_Linux, d)
}
