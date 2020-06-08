package append_dimensions

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

type Creds struct {
}

func (c *Creds) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	result := map[string]interface{}{}
	if len(agent.Global_Config.Credentials) != 0 {
		returnKey = CredsKey

		for k, v := range agent.Global_Config.Credentials {
			if k != agent.Role_Arn_Key {
				result[k] = v
			}
		}
	}
	returnVal = result
	return
}

func init() {
	c := new(Creds)
	RegisterRule(CredsKey, c)
}
