// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	// TestEKSDetector is used for unit testing EKS route
	testEKSDetector = func() (common.Detector, error) {
		cm := &v1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "aws-auth"},
			Data:       make(map[string]string),
		}
		return &common.EksDetector{Clientset: fake.NewSimpleClientset(cm)}, nil
	}
)

func TestTranslator(t *testing.T) {
	agent.Global_Config.Region = "us-east-1"
	testCases := map[string]struct {
		input           interface{}
		wantErrContains string
		detector        func() (common.Detector, error)
	}{
		"WithInvalidConfig": {
			input:           "",
			wantErrContains: "invalid json config",
		},
		"WithEmptyConfig": {
			input:           map[string]interface{}{},
			wantErrContains: "no valid pipelines",
		},
		"WithoutReceivers": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			wantErrContains: "no valid pipelines",
		},
		"WithMinimalConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
		},
		"WithAppSignalsMetricsEnabled": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				},
			},
			detector: testEKSDetector,
		},
		"WithAppSignalsTracesEnabled": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				},
			},
			detector: testEKSDetector,
		},
		"WithAppSignalsMetricsAndTracesEnabled": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				},
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				},
			},
			detector: testEKSDetector,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			common.NewDetector = testCase.detector
			translator.SetTargetPlatform("linux")
			got, err := Translate(testCase.input, "linux")
			if testCase.wantErrContains != "" {
				require.Error(t, err)
				assert.Nil(t, got)
				t.Log(err)
				assert.ErrorContains(t, err, testCase.wantErrContains)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

type testTranslator struct {
	id      component.ID
	version int
}

func (t testTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	return nil, nil
}

func (t testTranslator) ID() component.ID {
	return t.id
}

var _ common.Translator[*common.ComponentTranslators] = (*testTranslator)(nil)

func TestRegisterPipeline(t *testing.T) {
	original := &testTranslator{id: component.NewID("test"), version: 1}
	tm := common.NewTranslatorMap[*common.ComponentTranslators](original)
	assert.Equal(t, 0, registry.Len())
	first := &testTranslator{id: component.NewID("test"), version: 2}
	second := &testTranslator{id: component.NewID("test"), version: 3}
	RegisterPipeline(first, second)
	assert.Equal(t, 1, registry.Len())
	tm.Merge(registry)
	got, ok := tm.Get(component.NewID("test"))
	assert.True(t, ok)
	assert.Equal(t, second.version, got.(*testTranslator).version)
	assert.NotEqual(t, first.version, got.(*testTranslator).version)
	assert.NotEqual(t, original.version, got.(*testTranslator).version)
}
