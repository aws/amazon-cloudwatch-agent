// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migrate

import "errors"

func init() {
	AddRule(LogTargetRule)
}

//[agent]
//+  logtarget = "lumberjack"
func LogTargetRule(conf map[string]interface{}) error {
	agent, ok := conf["agent"].(map[string]interface{})
	if !ok {
		return errors.New("'agent' section missing from config")
	}

	// Do not change logtarget if it is manually set
	if _, ok := agent["logtarget"]; ok {
		return nil
	}

	agent["logtarget"] = "lumberjack"
	return nil
}
