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

	"github.com/aws/private-amazon-cloudwatch-agent-staging/test/generator/xray"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/test/util/awsservice"
)

const (
	agentRuntime          = 2 * time.Minute
	loadGeneratorInterval = 5 * time.Second
)

var (
	envMetaDataStrings = &(environment.MetaDataStrings{})
)

func init() {
	environment.RegisterEnvironmentMetaDataFlags(envMetaDataStrings)
}

func TestTraces(t *testing.T) {
	t.Run("Basic configuration for X-Ray", func(t *testing.T) {
		common.CopyFile(filepath.Join("testdata", "config.json"), common.ConfigOutputPath)
		startTime := time.Now()
		require.NoError(t, common.StartAgent(common.ConfigOutputPath, true, false))

		cfg := &xray.Config{
			Interval: loadGeneratorInterval,
			Annotations: map[string]interface{}{
				"test_type":   "basic",
				"instance_id": envMetaDataStrings.InstanceId,
				"commit_sha":  envMetaDataStrings.CwaCommitSha,
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
		}
		loadGen := xray.NewLoadGenerator(cfg)
		go func() {
			require.NoError(t, loadGen.Start(context.Background()), "load generator exited with error")
		}()
		time.Sleep(agentRuntime)
		loadGen.Stop()
		common.StopAgent()
		endTime := time.Now()
		t.Logf("Agent has been running for %s", endTime.Sub(startTime))
		time.Sleep(5 * time.Second)

		traceIDs, err := awsservice.GetTraceIDs(startTime, endTime, awsservice.FilterExpression(cfg.Annotations))
		require.NoError(t, err, "unable to get trace IDs")
		segments, err := awsservice.GetSegments(traceIDs)
		require.NoError(t, err, "unable to get segments")
		// 2 segments per
		assert.Len(t, segments, 20)
		validateSegments(t, segments, cfg)
	})
}

func validateSegments(t *testing.T, segments []types.Segment, cfg *xray.Config) {
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
