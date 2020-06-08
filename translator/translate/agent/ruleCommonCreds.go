package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

type CommonCreds struct {
}

const (
	Profile_Key                 = "profile"
	CredentialsFile_Key         = "shared_credential_file"
	CommonCredentialsSectionKey = "commoncredentials"
)

// Here we simply record the credentials map into the agent section(global).
// This credential map will be provided to the corresponding input and output plugins
// This should be applied before interpreting other component.
func (c *CommonCreds) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	result := map[string]interface{}{}

	// Read from common-toml
	ctx := context.CurrentContext()
	credsMapFromCtx := util.GetCredentials(ctx.Mode(), ctx.Credentials())

	if len(credsMapFromCtx) > 0 {
		keyMapping := map[string]string{
			commonconfig.CredentialProfile: Profile_Key,
			commonconfig.CredentialFile:    CredentialsFile_Key,
		}
		for k, v := range credsMapFromCtx {
			if mappedk, ok := keyMapping[k]; ok {
				result[mappedk] = v
			} else {
				result[k] = v
			}
		}
	}

	if _, ok := result[Profile_Key]; ok {
		if _, ok := result[CredentialsFile_Key]; !ok {
			// Use default credential path at present
			result[commonconfig.CredentialFile] = util.DetectCredentialsPath()
		}
	}

	Global_Config.Credentials = result

	return
}

func init() {
	c := new(CommonCreds)
	RegisterRule(CommonCredentialsSectionKey, c)
}
