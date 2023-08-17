// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package udp_logs

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	acit := NewTranslator()
	require.EqualValues(t, "udplog", acit.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *udplogreceiver.UDPLogConfig
		wantErr error
	}{
		"WithoutEmf": {
			input: map[string]interface{}{},
			wantErr: &common.MissingKeyError{
				ID:      acit.ID(),
				JsonKey: fmt.Sprintf("missing %s or udp service address", baseKey),
			},
		},
		"WithoutServiceAddress": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": map[string]interface{}{},
					},
				},
			},
			want: &udplogreceiver.UDPLogConfig{
				InputConfig: udp.Config{
					BaseConfig: udp.BaseConfig{
						ListenAddress: "0.0.0.0:25888",
					},
				},
			},
		},
		"TcpServiceAddress": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": map[string]interface{}{
							"service_address": "tcp:0.0.0.0:25888",
						},
					},
				},
			},
			wantErr: &common.MissingKeyError{
				ID:      acit.ID(),
				JsonKey: fmt.Sprintf("missing %s or udp service address", baseKey),
			},
		},
		"UdpServiceAddress": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": map[string]interface{}{
							"service_address": "udp:0.0.0.0:25888",
						},
					},
				},
			},
			want: &udplogreceiver.UDPLogConfig{
				InputConfig: udp.Config{
					BaseConfig: udp.BaseConfig{
						ListenAddress: "0.0.0.0:25888",
					},
				},
			},
		},
		"UdpDoubleSlashServiceAddress": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": map[string]interface{}{
							"service_address": "udp://localhost:25888",
						},
					},
				},
			},
			want: &udplogreceiver.UDPLogConfig{
				InputConfig: udp.Config{
					BaseConfig: udp.BaseConfig{
						ListenAddress: "localhost:25888",
					},
				},
			},
		},
		"UdpEmptyDoubleSlashServiceAddress": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": map[string]interface{}{
							"service_address": "udp://:25888",
						},
					},
				},
			},
			want: &udplogreceiver.UDPLogConfig{
				InputConfig: udp.Config{
					BaseConfig: udp.BaseConfig{
						ListenAddress: "0.0.0.0:25888",
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := acit.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*udplogreceiver.UDPLogConfig)
				require.True(t, ok)
				require.Equal(t, testCase.want.InputConfig.ListenAddress, gotCfg.InputConfig.ListenAddress)
				require.Equal(t, testCase.want.InputConfig.ListenAddress, gotCfg.InputConfig.ListenAddress)
			}
		})
	}
}
