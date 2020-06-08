package util

import "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"

type Creds struct {
	returnTargetKey string
}

// Grant the global creds(if exist)
func (c *Creds) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	if len(agent.Global_Config.Credentials) != 0 {
		returnKey = c.returnTargetKey
		returnVal = agent.Global_Config.Credentials
	}
	return
}

func GetCredsRule(returnTargetKey string) *Creds {
	c := new(Creds)
	c.returnTargetKey = returnTargetKey
	return c
}
