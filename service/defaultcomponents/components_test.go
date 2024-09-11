// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultcomponents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

const (
	receiversCount  = 7
	processorCount  = 11
	exportersCount  = 6
	extensionsCount = 4
)

func TestComponents(t *testing.T) {
	factories, err := Factories()
	assert.NoError(t, err)
	receivers := factories.Receivers
	assert.Len(t, receivers, receiversCount)

	nopType, _ := component.NewType("nop")

	awscontainerinsightreceiverType, _ := component.NewType("awscontainerinsightreceiver")
	awsxrayType, _ := component.NewType("awsxray")
	otlpType, _ := component.NewType("otlp")
	tcplogType, _ := component.NewType("tcplog")
	udplogType, _ := component.NewType("udplog")
	assert.NotNil(t, receivers[awscontainerinsightreceiverType])
	assert.NotNil(t, receivers[awsxrayType])
	assert.NotNil(t, receivers[otlpType])
	assert.NotNil(t, receivers[tcplogType])
	assert.NotNil(t, receivers[udplogType])
	assert.NotNil(t, receivers[nopType])

	processors := factories.Processors
	assert.Len(t, processors, processorCount)
	awsapplicationsignalsType, _ := component.NewType("awsapplicationsignals")
	awsentityType, _ := component.NewType("awsentity")
	batchType, _ := component.NewType("batch")
	cumulativetodeltaType, _ := component.NewType("cumulativetodelta")
	ec2taggerType, _ := component.NewType("ec2tagger")
	gpuattributesType, _ := component.NewType("gpuattributes")
	metricstransformType, _ := component.NewType("metricstransform")
	transformType, _ := component.NewType("transform")
	assert.NotNil(t, processors[awsapplicationsignalsType])
	assert.NotNil(t, processors[awsentityType])
	assert.NotNil(t, processors[batchType])
	assert.NotNil(t, processors[cumulativetodeltaType])
	assert.NotNil(t, processors[ec2taggerType])
	assert.NotNil(t, processors[gpuattributesType])
	assert.NotNil(t, processors[metricstransformType])
	assert.NotNil(t, processors[transformType])

	exporters := factories.Exporters
	assert.Len(t, exporters, exportersCount)
	awscloudwatchlogsType, _ := component.NewType("awscloudwatchlogs")
	awsemfType, _ := component.NewType("awsemf")
	awscloudwatchType, _ := component.NewType("awscloudwatch")
	debugType, _ := component.NewType("debug")
	assert.NotNil(t, exporters[awscloudwatchlogsType])
	assert.NotNil(t, exporters[awsemfType])
	assert.NotNil(t, exporters[awsemfType])
	assert.NotNil(t, exporters[awscloudwatchType])
	assert.NotNil(t, exporters[debugType])
	assert.NotNil(t, exporters[nopType])

	extensions := factories.Extensions
	assert.Len(t, extensions, extensionsCount)
	agenthealthType, _ := component.NewType("agenthealth")
	awsproxyType, _ := component.NewType("awsproxy")
	entitystore, _ := component.NewType("entitystore")
	server, _ := component.NewType("server")
	assert.NotNil(t, extensions[agenthealthType])
	assert.NotNil(t, extensions[awsproxyType])
	assert.NotNil(t, extensions[entitystore])
	assert.NotNil(t, extensions[server])
}
