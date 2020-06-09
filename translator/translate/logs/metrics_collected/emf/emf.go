package emf

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected"
)

//
// Need to import new rule package in src/translator/totomlconfig/toTomlConfig.go
//

//
//   "emf" : {
//       "service_address": "udp://127.0.0.1:25888"
//   }
//
const SectionKey = "emf"

var ChildRule = map[string]translator.Rule{}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type EMF struct {
}

func (obj *EMF) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If exists, process it
		//Check if there are some config entry with rules applied
		if sectionMap, ok := m[SectionKey].(map[string]interface{}); ok && len(sectionMap) == 0 {
			// not configured
			defaultEndpointSuffix := "://127.0.0.1:25888"
			if context.CurrentContext().RunInContainer() {
				defaultEndpointSuffix = "://:25888"
			}
			resArray = []interface{}{
				map[string]interface{}{
					"service_address": "udp" + defaultEndpointSuffix,
					"data_format":     "emf",
					"name_override":   "emf",
				},
				map[string]interface{}{
					"service_address": "tcp" + defaultEndpointSuffix,
					"data_format":     "emf",
					"name_override":   "emf",
				},
			}
		} else {
			result = translator.ProcessRuleToApply(m[SectionKey], ChildRule, result)
			resArray = append(resArray, result)
		}
		returnKey = "socket_listener"
		returnVal = resArray
	}
	return
}

func init() {
	obj := new(EMF)
	parent.RegisterLinuxRule(SectionKey, obj)
	parent.RegisterWindowsRule(SectionKey, obj)
}
