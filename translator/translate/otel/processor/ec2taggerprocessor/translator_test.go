// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2taggerprocessor

import (
	"sort"
	"testing"
	"time"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/processors/ec2tagger"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	etpTranslator := NewTranslator()
	require.EqualValues(t, "ec2tagger", etpTranslator.Type())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *ec2tagger.Config
		wantErr error
	}{
		"MissingAppendDimensionsConfig": {
			wantErr: &common.MissingKeyError{
				Type:    "ec2tagger",
				JsonKey: common.ConfigKey("metrics", "append_dimensions"),
			},
		},
		"FullEc2TaggerProcessorConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"append_dimensions": map[string]interface{}{
						"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
						"ImageId":              "${aws:ImageId}",
						"InstanceId":           "${aws:InstanceId}",
						"InstanceType":         "${aws:InstanceType}",
					},
				},
			},
			want: &ec2tagger.Config{
				RefreshIntervalSeconds: 0 * time.Second,
				EC2MetadataTags:        []string{"ImageId", "InstanceId", "InstanceType"},
				EC2InstanceTagKeys:     []string{"AutoScalingGroupName"},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			got, err := etpTranslator.Translate(conf)
			require.Equal(t, tc.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*ec2tagger.Config)
				require.True(t, ok)
				require.Equal(t, tc.want.RefreshIntervalSeconds, gotCfg.RefreshIntervalSeconds)
				sort.Strings(gotCfg.EC2MetadataTags)
				require.Equal(t, tc.want.EC2MetadataTags, gotCfg.EC2MetadataTags)
				require.Equal(t, tc.want.EC2InstanceTagKeys, gotCfg.EC2InstanceTagKeys)
			}
		})
	}
}
