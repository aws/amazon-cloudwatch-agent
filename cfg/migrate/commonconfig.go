package migrate

import (
	"errors"
	"log"
	"os"
)

func init() {
	AddRule(CommonConfigRule)
}

var commonConfigs = map[string]string{
	"aws_ca_bundle": "AWS_CA_BUNDLE",
	"http_proxy":    "HTTP_PROXY",
	"https_proxy":   "HTTPS_PROXY",
	"no_proxy":      "NO_PROXY",
}

//  [agent]
//-   aws_ca_bundle = "/etc/test/ca_bundle.pem"
//-   http_proxy = "http://127.0.0.1:3280"
//-   https_proxy = "https://127.0.0.1:3280"
//-   no_proxy = "254.1.1.1"
func CommonConfigRule(conf map[string]interface{}) error {
	agent, ok := conf["agent"].(map[string]interface{})
	if !ok {
		return errors.New("'agent' section missing from config")
	}

	for cfg, env := range commonConfigs {
		if value, ok := agent[cfg].(string); ok {
			log.Printf("I! %s \"%s\" is set!\n", env, value)
			os.Setenv(env, value)
		}
		delete(agent, cfg)
	}

	return nil
}
