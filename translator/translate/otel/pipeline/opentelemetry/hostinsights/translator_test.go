// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostinsights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/hostmetrics"
)

func TestHostInsightsTranslator(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_LINUX)
	tt := NewTranslator()
	assert.EqualValues(t, "metrics/host_insights", tt.ID().String())

	testCases := map[string]struct {
		input         map[string]interface{}
		wantErr       error
		expectProcess bool
	}{
		"WithNilConf": {
			input:   nil,
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: hostInsightsKey + " or " + common.DatabaseInsightsConfigKey},
		},
		"WithoutHostInsightsKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: hostInsightsKey + " or " + common.DatabaseInsightsConfigKey},
		},
		"WithHostInsightsKey": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"host_insights": map[string]interface{}{},
					},
				},
			},
		},
		"WithDatabaseInsightsOnlyKey": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"database_insights": map[string]interface{}{
							"postgresql": []interface{}{
								map[string]interface{}{"endpoint": "localhost:5432", "username": "cwagent"},
							},
						},
					},
				},
			},
			expectProcess: true,
		},
		"WithBothKeys": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"host_insights": map[string]interface{}{},
						"database_insights": map[string]interface{}{
							"postgresql": []interface{}{
								map[string]interface{}{"endpoint": "localhost:5432", "username": "cwagent"},
							},
						},
					},
				},
			},
			expectProcess: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var conf *confmap.Conf
			if tc.input != nil {
				conf = confmap.NewFromStringMap(tc.input)
			}
			got, err := tt.Translate(conf)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, 1, got.Receivers.Len())
				assert.Equal(t, 0, got.Processors.Len())
				assert.Equal(t, 1, got.Exporters.Len())
				assert.Equal(t, 0, got.Extensions.Len())
				assert.Equal(t, 1, got.Connectors.Len())
				assert.Equal(t, "hostmetrics/opentelemetry", got.Receivers.Keys()[0].String())
				assert.Equal(t, "forward/opentelemetry", got.Exporters.Keys()[0].String())
				assert.Equal(t, "forward/opentelemetry", got.Connectors.Keys()[0].String())

				if tc.expectProcess {
					// Verify process scraper is configured for DBI
					rcvTranslator, ok := got.Receivers.Get(component.NewIDWithName(component.MustNewType("hostmetrics"), "opentelemetry"))
					require.True(t, ok)
					rcvCfg, err := rcvTranslator.Translate(conf)
					require.NoError(t, err)
					hmCfg := rcvCfg.(*hostmetrics.Config)
					processCfg, exists := hmCfg.Scrapers["process"]
					assert.True(t, exists, "expected process scraper")
					include := processCfg["include"].(map[string]any)
					assert.Equal(t, "regexp", include["match_type"])
					assert.Equal(t, []string{"postgres.*"}, include["names"])
				}
			}
		})
	}
}
