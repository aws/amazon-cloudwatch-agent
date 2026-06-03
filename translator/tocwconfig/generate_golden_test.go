// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tocwconfig

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toyamlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestGenerateContainerInsightsGolden(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)

	agent.Global_Config = *new(agent.Agent)
	translator.SetTargetPlatform("linux")

	data, err := os.ReadFile("./sampleConfig/container_insights_config.json")
	require.NoError(t, err)

	var input interface{}
	require.NoError(t, json.Unmarshal(data, &input))

	yamlConfig, err := cmdutil.TranslateJsonMapToYamlConfigNoValidation(input)
	require.NoError(t, err)

	yamlStr := toyamlconfig.ToYamlConfig(yamlConfig)
	err = os.WriteFile("./sampleConfig/container_insights_config.yaml", []byte(yamlStr), 0600)
	require.NoError(t, err)
	t.Log("Golden file generated successfully")
}
