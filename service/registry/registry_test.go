// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestRegistry(t *testing.T) {
	Register(WithReceiver(receivertest.NewNopFactory()), WithProcessor(processortest.NewNopFactory()))
	Register(WithExporter(exportertest.NewNopFactory()), WithExtension(extensiontest.NewNopFactory()))
	assert.Len(t, Options(), 4)
	got := otelcol.Factories{}
	for _, apply := range Options() {
		apply(&got)
	}
	nop, _ := component.NewType("nop")
	assert.NotNil(t, got.Receivers[nop])
	assert.NotNil(t, got.Processors[nop])
	assert.NotNil(t, got.Exporters[nop])
	assert.NotNil(t, got.Extensions[nop])
	assert.Len(t, got.Receivers, 1)
	origReceiver := got.Receivers[nop]
	Register(WithReceiver(receivertest.NewNopFactory()))
	for _, apply := range Options() {
		apply(&got)
	}
	newReceiver := got.Receivers[nop]
	assert.NotEqual(t, origReceiver, newReceiver)
	assert.Len(t, got.Receivers, 1)
	Reset()
	assert.Nil(t, Options())
}
