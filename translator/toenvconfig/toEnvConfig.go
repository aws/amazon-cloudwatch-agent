package toenvconfig

import (
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/csm"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

func ToEnvConfig(jsonConfigValue map[string]interface{}) []byte {
	envVars := make(map[string]string)
	// If csm has a configuration section, then also turn on csm for the agent itself
	if _, ok := jsonConfigValue[csm.JSONSectionKey]; ok {
		envVars[envconfig.AWS_CSM_ENABLED] = "TRUE"
	}

	proxy := util.GetHttpProxy(context.CurrentContext().Proxy())
	if len(proxy) > 0 {
		envVars[envconfig.HTTP_PROXY] = proxy[commonconfig.HttpProxy]
	}

	proxy = util.GetHttpsProxy(context.CurrentContext().Proxy())
	if len(proxy) > 0 {
		envVars[envconfig.HTTPS_PROXY] = proxy[commonconfig.HttpsProxy]
	}

	proxy = util.GetNoProxy(context.CurrentContext().Proxy())
	if len(proxy) > 0 {
		envVars[envconfig.NO_PROXY] = proxy[commonconfig.NoProxy]
	}

	sslConfig := util.GetSSL(context.CurrentContext().SSL())
	if len(sslConfig) > 0 {
		envVars[envconfig.AWS_CA_BUNDLE] = sslConfig[commonconfig.CABundlePath]
	}

	bytes, err := json.MarshalIndent(envVars, "", "\t")
	if err != nil {
		panic(fmt.Sprintf("Failed to create json map for environment variables. Reason: %s \n", err.Error()))
	}
	return bytes
}
