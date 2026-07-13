// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetrics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
		extensions []string
	}
	tt := NewTranslator()
	assert.Equal(t, "metrics/systemmetrics", tt.ID().String())

	testCases := map[string]struct {
		input          map[string]interface{}
		runInContainer bool
		kubernetes     bool
		envEnabled     string // "true", "false", or "" (not set)
		imdsAvailable  bool
		onPrem         bool
		want           *want
		wantErr        error
	}{
		"WithEnvDisabled": {
			input:         map[string]interface{}{},
			envEnabled:    "false",
			imdsAvailable: true,
			wantErr:       &common.MissingKeyError{ID: tt.ID(), JsonKey: common.SystemMetricsEnabledConfigKey},
		},
		"WithEnvEnabled": {
			input:         map[string]interface{}{},
			envEnabled:    "true",
			imdsAvailable: true,
			want: &want{
				receivers:  []string{"systemmetrics"},
				processors: []string{"ec2tagger/systemmetrics", "batch/systemmetrics"},
				exporters:  []string{"awscloudwatch/systemmetrics"},
				extensions: []string{"agenthealth/metrics"},
			},
		},
		"WithEnvEnabledOverridesContainer": {
			input:          map[string]interface{}{},
			envEnabled:     "true",
			runInContainer: true,
			imdsAvailable:  true,
			want: &want{
				receivers:  []string{"systemmetrics"},
				processors: []string{"ec2tagger/systemmetrics", "batch/systemmetrics"},
				exporters:  []string{"awscloudwatch/systemmetrics"},
				extensions: []string{"agenthealth/metrics"},
			},
		},
		"WithJsonDisabled": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"system_metrics_enabled": false,
				},
			},
			imdsAvailable: true,
			wantErr:       &common.MissingKeyError{ID: tt.ID(), JsonKey: common.SystemMetricsEnabledConfigKey},
		},
		"WithJsonEnabled": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"system_metrics_enabled": true,
				},
			},
			imdsAvailable: true,
			want: &want{
				receivers:  []string{"systemmetrics"},
				processors: []string{"ec2tagger/systemmetrics", "batch/systemmetrics"},
				exporters:  []string{"awscloudwatch/systemmetrics"},
				extensions: []string{"agenthealth/metrics"},
			},
		},
		"WithRunInContainer": {
			input:          map[string]interface{}{},
			runInContainer: true,
			imdsAvailable:  true,
			wantErr:        &common.MissingKeyError{ID: tt.ID(), JsonKey: common.SystemMetricsEnabledConfigKey},
		},
		"WithKubernetes": {
			input:         map[string]interface{}{},
			kubernetes:    true,
			imdsAvailable: true,
			wantErr:       &common.MissingKeyError{ID: tt.ID(), JsonKey: common.SystemMetricsEnabledConfigKey},
		},
		"WithIMDSUnavailable": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"system_metrics_enabled": true,
				},
			},
			imdsAvailable: false,
			want: &want{
				receivers:  []string{"systemmetrics"},
				processors: []string{"batch/systemmetrics"},
				exporters:  []string{"awscloudwatch/systemmetrics"},
				extensions: []string{"agenthealth/metrics"},
			},
		},
		"WithOnPrem": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"system_metrics_enabled": true,
				},
			},
			imdsAvailable: true,
			onPrem:        true,
			want: &want{
				receivers:  []string{"systemmetrics"},
				processors: []string{"batch/systemmetrics"},
				exporters:  []string{"awscloudwatch/systemmetrics"},
				extensions: []string{"agenthealth/metrics"},
			},
		},
		// TODO: add WithUnrecognizedHost once host detection paths (/apollo, /etc/image-id,
		// /etc/os-release) are injectable, so the test doesn't depend on the environment it runs in.
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			context.ResetContext()
			orig := IsIMDSAvailable
			origOnPrem := isOnPrem
			imds := tc.imdsAvailable
			onPrem := tc.onPrem
			IsIMDSAvailable = func() bool { return imds }
			isOnPrem = func() bool { return onPrem }
			t.Cleanup(func() {
				IsIMDSAvailable = orig
				isOnPrem = origOnPrem
			})

			if tc.envEnabled != "" {
				t.Setenv(envconfig.SystemMetricsEnabled, tc.envEnabled)
			}
			if tc.runInContainer {
				context.CurrentContext().SetRunInContainer(true)
			}
			if tc.kubernetes {
				t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
			}
			conf := confmap.NewFromStringMap(tc.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, tc.wantErr, err)
			if tc.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tc.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, tc.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, tc.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, tc.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}

func TestParseKeyFromFile(t *testing.T) {
	tmp := t.TempDir()

	f1 := filepath.Join(tmp, "noquotes")
	require.NoError(t, os.WriteFile(f1, []byte("image_name=amzn2-foo-naws-bar\nimage_version=2.0\n"), 0600))
	assert.Equal(t, "amzn2-foo-naws-bar", parseKeyFromFile(f1, "image_name"))
	assert.Equal(t, "2.0", parseKeyFromFile(f1, "image_version"))
	assert.Equal(t, "", parseKeyFromFile(f1, "missing_key"))

	f2 := filepath.Join(tmp, "quoted")
	require.NoError(t, os.WriteFile(f2, []byte("VARIANT=\"internal\"\nNAME=\"Amazon Linux\"\n"), 0600))
	assert.Equal(t, "internal", parseKeyFromFile(f2, "VARIANT"))
	assert.Equal(t, "Amazon Linux", parseKeyFromFile(f2, "NAME"))

	assert.Equal(t, "", parseKeyFromFile(filepath.Join(tmp, "nonexistent"), "key"))
}

func TestCheckImageIDWithMarkers(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		expected  bool
	}{
		{"amzn2 naws", "amzn2-foo-naws-bar", true},
		{"amzn2 internal", "amzn2-internal-baz", true},
		{"al2023 naws", "al2023-naws-qux", true},
		{"al2023 internal", "al2023-internal-quux", true},
		{"al2-unified", "al2-unified-corge", true},
		{"al2023 no marker", "al2023-foo-bar", false},
		{"amzn2 no marker", "amzn2-foo-bar", false},
		// wrong prefix with valid marker — should NOT match
		{"ubuntu with naws", "ubuntu-naws-22.04", false},
		{"centos with internal", "centos-internal-7", false},
		// unified without al2 prefix — should NOT match
		{"random unified", "fedora-unified-39", false},
		// empty
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, matchesImageNameMarker(tc.imageName))
		})
	}
}

func TestCheckOSReleaseVariant(t *testing.T) {
	tmp := t.TempDir()

	f1 := filepath.Join(tmp, "internal")
	require.NoError(t, os.WriteFile(f1, []byte("NAME=\"Amazon Linux\"\nVARIANT=\"internal\"\n"), 0600))
	assert.Equal(t, "Amazon Linux", parseKeyFromFile(f1, "NAME"))
	assert.Equal(t, "internal", parseKeyFromFile(f1, "VARIANT"))

	f2 := filepath.Join(tmp, "public")
	require.NoError(t, os.WriteFile(f2, []byte("NAME=\"Amazon Linux\"\nVERSION=\"2023\"\n"), 0600))
	assert.Equal(t, "", parseKeyFromFile(f2, "VARIANT"))

	f3 := filepath.Join(tmp, "wrong-name")
	require.NoError(t, os.WriteFile(f3, []byte("NAME=\"Ubuntu\"\nVARIANT=\"internal\"\n"), 0600))
	assert.Equal(t, "Ubuntu", parseKeyFromFile(f3, "NAME"))
	assert.Equal(t, "internal", parseKeyFromFile(f3, "VARIANT"))
}
