package gpu

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

var ChildRule = map[string]translator.Rule{}

//
//	"nvidia_gpu": {
//		"measurement": [
//			"utilization_gpu",
//			"temperature_gpu"
//		],
//      "metrics_collection_interval": 60
//	}
//

// SectionKey_Nvidia_GPU metrics name in user config to opt in Nvidia GPU metrics
const SectionKey_Nvidia_GPU = "nvidia_gpu"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey_Nvidia_GPU + "/"
	return curPath
}

func RegisterRule(fieldname string, r translator.Rule) {
	ChildRule[fieldname] = r
}

type NvidiaSmi struct {
}

func (n *NvidiaSmi) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArr := []interface{}{}
	result := map[string]interface{}{}
	// nvidia_gpu is not the real telegraf plugin's name, need to register the real plugin name to enable it.
	telegrafPluginName := config.GetRealPluginName(SectionKey_Nvidia_GPU)
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKey_Nvidia_GPU]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		/*
		  In JSON config file, it represent as "nvidia_gpu" : {//specification config information}
		  To check the specification config entry
		*/
		//Check if there are any config entry with rules applied
		result = translator.ProcessRuleToApply(m[SectionKey_Nvidia_GPU], ChildRule, result)
		//Process common config, like measurement
		hasValidMetric := util.ProcessLinuxCommonConfig(m[SectionKey_Nvidia_GPU], telegrafPluginName, GetCurPath(), result)
		if hasValidMetric {
			resArr = append(resArr, result)
			returnKey = telegrafPluginName
			returnVal = resArr
		} else {
			returnKey = ""
		}
	}
	return
}

func init() {
	n := new(NvidiaSmi)
	parent.RegisterLinuxRule(SectionKey_Nvidia_GPU, n)
	parent.RegisterDarwinRule(SectionKey_Nvidia_GPU, n)
	//parent.RegisterWindowsRule(SectionKey_Nvidia_GPU, n)
}
