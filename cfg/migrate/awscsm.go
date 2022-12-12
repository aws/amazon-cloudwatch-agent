// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

import (
	"errors"
	"os"
)

func init() {
	AddRule(AwsCsmRule)
}

//  [agent]
//-   enable_csm_reflection = true
//    [[inputs.awscsm_listener]]
//+     data_format = "aws_csm"
func AwsCsmRule(conf map[string]interface{}) error {
	agent, ok := conf["agent"].(map[string]interface{})
	if !ok {
		return errors.New("'agent' section missing from config")
	}

	if enabled, ok := agent["enable_csm_reflection"].(bool); ok && enabled {
		os.Setenv("AWS_CSM_ENABLED", "TRUE")
	}
	delete(agent, "enable_csm_reflection")

	csms := getItem(conf, "inputs", "awscsm_listener")

	for _, csm := range csms {
		csm["data_format"] = "aws_csm"
	}

	return nil
}
