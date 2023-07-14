// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package trace

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent-test/environment"
	"github.com/aws/amazon-cloudwatch-agent-test/util/common"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/test/generator"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/test/generator/otlp"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/test/generator/xray"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/test/util/awsservice"
)

const (
	agentRuntime          = 2 * time.Minute
	loadGeneratorInterval = 5 * time.Second
)

func init() {
	environment.RegisterEnvironmentMetaDataFlags()
}

func TestTraces(t *testing.T) {
	env := environment.GetEnvironmentMetaData()
	testCases := map[string]struct {
		agentConfigPath  string
		newLoadGenerator func(*generator.Config) generator.Generator
		generatorConfig  *generator.Config
	}{
		"WithXray/Simple": {
			agentConfigPath:  filepath.Join("testdata", "xray-config.json"),
			newLoadGenerator: xray.NewLoadGenerator,
			generatorConfig: &generator.Config{
				Interval: loadGeneratorInterval,
				Annotations: map[string]interface{}{
					"test_type":   "simple_xray",
					"instance_id": env.InstanceId,
					"commit_sha":  env.CwaCommitSha,
				},
				Metadata: map[string]map[string]interface{}{
					"default": {
						"nested": map[string]interface{}{
							"key": "value",
						},
					},
					"custom_namespace": {
						"custom_key": "custom_value",
					},
				},
			},
		},
		"WithOTLP/Simple": {
			agentConfigPath:  filepath.Join("testdata", "otlp-config.json"),
			newLoadGenerator: otlp.NewLoadGenerator,
			generatorConfig: &generator.Config{
				Interval: loadGeneratorInterval,
				Annotations: map[string]interface{}{
					"test_type":   "simple_otlp",
					"instance_id": env.InstanceId,
					"commit_sha":  env.CwaCommitSha,
				},
				Metadata: map[string]map[string]interface{}{
					"default": {
						"custom_key": "custom_value",
					},
				},
				Attributes: []attribute.KeyValue{
					attribute.String("custom_key", "custom_value"),
					attribute.String("test_type", "simple_otlp"),
					attribute.String("instance_id", env.InstanceId),
					attribute.String("commit_sha", env.CwaCommitSha),
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			common.CopyFile(testCase.agentConfigPath, common.ConfigOutputPath)
			startTime := time.Now()
			require.NoError(t, common.StartAgent(common.ConfigOutputPath, true, false))

			loadGen := testCase.newLoadGenerator(testCase.generatorConfig)
			go func() {
				require.NoError(t, loadGen.Start(context.Background()), "load generator exited with error")
			}()
			time.Sleep(agentRuntime)
			loadGen.Stop()
			common.StopAgent()
			endTime := time.Now()
			t.Logf("Agent has been running for %s", endTime.Sub(startTime))
			time.Sleep(5 * time.Second)

			traceIDs, err := awsservice.GetTraceIDs(startTime, endTime, awsservice.FilterExpression(testCase.generatorConfig.Annotations))
			require.NoError(t, err, "unable to get trace IDs")
			segments, err := awsservice.GetSegments(traceIDs)
			require.NoError(t, err, "unable to get segments")

			assert.True(t, len(segments) >= 20)
			validateSegments(t, segments, testCase.generatorConfig)
		})
	}
}

func validateSegments(t *testing.T, segments []types.Segment, cfg *generator.Config) {
	t.Helper()
	for _, segment := range segments {
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(*segment.Document), &result))
		if _, ok := result["parent_id"]; ok {
			// skip subsegments
			continue
		}
		annotations, ok := result["annotations"]
		assert.True(t, ok, "missing annotations")
		assert.True(t, reflect.DeepEqual(annotations, cfg.Annotations), "mismatching annotations")
		metadataByNamespace, ok := result["metadata"].(map[string]interface{})
		assert.True(t, ok, "missing metadata")
		for namespace, wantMetadata := range cfg.Metadata {
			var gotMetadata map[string]interface{}
			gotMetadata, ok = metadataByNamespace[namespace].(map[string]interface{})
			assert.Truef(t, ok, "missing metadata in namespace: %s", namespace)
			for key, wantValue := range wantMetadata {
				var gotValue interface{}
				gotValue, ok = gotMetadata[key]
				assert.Truef(t, ok, "missing expected metadata key: %s", key)
				assert.Truef(t, reflect.DeepEqual(gotValue, wantValue), "mismatching values for key (%s):\ngot\n\t%v\nwant\n\t%v", key, gotValue, wantValue)
			}
		}
	}
}
