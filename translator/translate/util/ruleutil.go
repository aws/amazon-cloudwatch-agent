package util

import "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"

type Region struct {
	returnTargetKey string
}

// Grant the global creds(if exist)
func (r *Region) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = r.returnTargetKey
	returnVal = map[string]interface{}{"region": agent.Global_Config.Region}
	return
}

func GetRegionRule(returnTargetKey string) *Region {
	r := new(Region)
	r.returnTargetKey = returnTargetKey
	return r
}
