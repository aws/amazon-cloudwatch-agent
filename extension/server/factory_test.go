// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExtension(t *testing.T) {
	cfg := &Config{}
	got, err := NewFactory().Create(context.Background(), extensiontest.NewNopSettings(component.MustNewType("server")), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestCreateWithConfig(t *testing.T) {
	cfg := &Config{ListenAddress: ":8080", TLSCertPath: "./testdata/example-server-cert.pem",
		TLSKeyPath: "./testdata/example-server-key.pem",
		TLSCAPath:  "./testdata/example-CA-cert.pem"}
	got, err := NewFactory().Create(context.Background(), extensiontest.NewNopSettings(component.MustNewType("server")), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}
