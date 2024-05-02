// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2taggerprocessor

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	etpTranslator := NewTranslator()
	require.EqualValues(t, "ec2tagger", etpTranslator.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *ec2tagger.Config
		wantErr error
	}{
		"MissingAppendDimensionsConfig": {
			wantErr: &common.MissingKeyError{
				ID:      etpTranslator.ID(),
				JsonKey: ec2taggerKey,
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
		"WithDiskAppendDimensions": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"append_dimensions": map[string]interface{}{
						"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
						"ImageId":              "${aws:ImageId}",
						"InstanceId":           "${aws:InstanceId}",
						"InstanceType":         "${aws:InstanceType}",
					},
					"metrics_collected": map[string]interface{}{
						"disk": map[string]interface{}{
							"append_dimensions": map[string]interface{}{
								"VolumeId": "${aws:VolumeId}",
							},
						},
					},
				},
			},
			want: &ec2tagger.Config{
				RefreshIntervalSeconds: 0 * time.Second,
				EC2MetadataTags:        []string{"ImageId", "InstanceId", "InstanceType"},
				EC2InstanceTagKeys:     []string{"AutoScalingGroupName"},
				DiskDeviceTagKey:       "device",
				EBSDeviceKeys:          []string{"*"},
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
				require.Equal(t, tc.want.DiskDeviceTagKey, gotCfg.DiskDeviceTagKey)
				require.Equal(t, tc.want.EBSDeviceKeys, gotCfg.EBSDeviceKeys)
			}
		})
	}
}
