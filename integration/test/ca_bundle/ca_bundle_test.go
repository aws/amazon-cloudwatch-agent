// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package ca_bundle

import (
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
)

const configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
const commonConfigOutputPath = "/opt/aws/amazon-cloudwatch-agent/etc/common-config.toml"
const configJSON = "/config.json"
const commonConfigTOML = "/common-config.toml"
const targetString = "x509: certificate signed by unknown authority"

//Let the agent run for 1 minutes. This will give agent enough time to call server
const agentRuntime = 1 * time.Minute

type input struct {
	findTarget bool
	dataInput  string
}

//Must run this test with parallel 1 since this will fail if more than one test is running at the same time
//This test uses a pem file created for the local stack endpoint to be able to connect via ssl
func TestBundle(t *testing.T) {

	parameters := []input{
		//Use the system pem ca bundle  + local stack pem file ssl should connect thus target string not found
		{dataInput: "resources/integration/ssl/with/combine/bundle", findTarget: false},
		//Do not look for ca bundle with http connection should connect thus target string not found
		{dataInput: "resources/integration/ssl/without/bundle/http", findTarget: false},
		//Use the system pem ca bundle ssl should not connect thus target string found
		{dataInput: "resources/integration/ssl/with/original/bundle", findTarget: true},
		//Do not look for ca bundle should not connect thus target string found
		{dataInput: "resources/integration/ssl/without/bundle", findTarget: true},
	}

	for _, parameter := range parameters {
		//before test run
		log.Printf("resource file location %s find target %t", parameter.dataInput, parameter.findTarget)
		t.Run(fmt.Sprintf("resource file location %s find target %t", parameter.dataInput, parameter.findTarget), func(t *testing.T) {
			test.ReplaceLocalStackHostName(parameter.dataInput + configJSON)
			test.CopyFile(parameter.dataInput+configJSON, configOutputPath)
			test.CopyFile(parameter.dataInput+commonConfigTOML, commonConfigOutputPath)
			test.StartAgent(configOutputPath)
			time.Sleep(agentRuntime)
			log.Printf("Agent has been running for : %s", agentRuntime.String())
			test.StopAgent()
			output := test.ReadAgentOutput(agentRuntime)
			containsTarget := outputLogContainsTarget(output)
			if (parameter.findTarget && !containsTarget) || (!parameter.findTarget && containsTarget) {
				t.Errorf("Find target is %t contains target is %t", parameter.findTarget, containsTarget)
			}
		})
	}
}

func outputLogContainsTarget(output string) bool {
	log.Printf("Log file %s", output)
	contains := strings.Contains(output, targetString)
	log.Printf("Log file contains target string %t", contains)
	return contains
}
