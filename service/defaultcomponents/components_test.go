// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultcomponents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

const (
	receiversCount  = 6
	processorCount  = 11
	exportersCount  = 6
	extensionsCount = 3
)

func TestComponents(t *testing.T) {
	factories, err := Factories()
	assert.NoError(t, err)
	receivers := factories.Receivers
	assert.Len(t, receivers, receiversCount)
	assert.NotNil(t, receivers[component.MustNewType("awscontainerinsightreceiver")])
	assert.NotNil(t, receivers[component.MustNewType("awsxray")])
	assert.NotNil(t, receivers[component.MustNewType("otlp")])
	assert.NotNil(t, receivers[component.MustNewType("tcplog")])
	assert.NotNil(t, receivers[component.MustNewType("udplog")])

	processors := factories.Processors
	assert.Len(t, processors, processorCount)
	assert.NotNil(t, processors[component.MustNewType("awsapplicationsignals")])
	assert.NotNil(t, processors[component.MustNewType("batch")])
	assert.NotNil(t, processors[component.MustNewType("cumulativetodelta")])
	assert.NotNil(t, processors[component.MustNewType("ec2tagger")])
	assert.NotNil(t, processors[component.MustNewType("gpuattributes")])
	assert.NotNil(t, processors[component.MustNewType("metricstransform")])
	assert.NotNil(t, processors[component.MustNewType("rollup")])
	assert.NotNil(t, processors[component.MustNewType("transform")])

	exporters := factories.Exporters
	assert.Len(t, exporters, exportersCount)
	assert.NotNil(t, exporters[component.MustNewType("awscloudwatch")])
	assert.NotNil(t, exporters[component.MustNewType("awscloudwatchlogs")])
	assert.NotNil(t, exporters[component.MustNewType("awsemf")])
	assert.NotNil(t, exporters[component.MustNewType("debug")])
	assert.NotNil(t, exporters[component.MustNewType("prometheusremotewrite")])

	extensions := factories.Extensions
	assert.Len(t, extensions, extensionsCount)
	assert.NotNil(t, extensions[component.MustNewType("agenthealth")])
	assert.NotNil(t, extensions[component.MustNewType("awsproxy")])
	assert.NotNil(t, extensions[component.MustNewType("sigv4auth")])
}
