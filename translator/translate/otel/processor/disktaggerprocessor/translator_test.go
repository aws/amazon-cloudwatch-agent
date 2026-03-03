// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktaggerprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/disktagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestIsSet(t *testing.T) {
	tests := map[string]struct {
		input map[string]interface{}
		want  bool
	}{
		"Nil": {want: false},
		"Empty": {
			input: map[string]interface{}{},
			want:  false,
		},
		"LegacyVolumeId": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"disk": map[string]interface{}{
							"append_dimensions": map[string]interface{}{
								"VolumeId": "${aws:VolumeId}",
							},
						},
					},
				},
			},
			want: true,
		},
		"OTelDiskId": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"disk": map[string]interface{}{
							"append_dimensions": map[string]interface{}{
								"VolumeId": "${disk.id}",
							},
						},
					},
				},
			},
			want: true,
		},
		"DiskIdKey": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"disk": map[string]interface{}{
							"append_dimensions": map[string]interface{}{
								"DiskId": "${disk.id}",
							},
						},
					},
				},
			},
			want: true,
		},
		"UnsupportedValue": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"disk": map[string]interface{}{
							"append_dimensions": map[string]interface{}{
								"VolumeId": "static-value",
							},
						},
					},
				},
			},
			want: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var conf *confmap.Conf
			if tc.input != nil {
				conf = confmap.NewFromStringMap(tc.input)
			}
			assert.Equal(t, tc.want, IsSet(conf))
		})
	}
}

func TestTranslator(t *testing.T) {
	tr := NewTranslator()
	require.EqualValues(t, "disktagger", tr.ID().String())

	tests := map[string]struct {
		input   map[string]interface{}
		wantErr bool
	}{
		"MissingConfig": {
			input:   map[string]interface{}{},
			wantErr: true,
		},
		"WithVolumeId": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"disk": map[string]interface{}{
							"append_dimensions": map[string]interface{}{
								"VolumeId": "${aws:VolumeId}",
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			got, err := tr.Translate(conf)
			if tc.wantErr {
				require.Error(t, err)
				var missing *common.MissingKeyError
				require.ErrorAs(t, err, &missing)
				return
			}
			require.NoError(t, err)
			cfg, ok := got.(*disktagger.Config)
			require.True(t, ok)
			assert.Equal(t, 5*time.Minute, cfg.RefreshInterval)
			assert.Equal(t, "device", cfg.DiskDeviceTagKey)
		})
	}
}
