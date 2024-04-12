// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultcomponents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	receiversCount  = 5
	processorCount  = 8
	exportersCount  = 5
	extensionsCount = 2
)

func TestComponents(t *testing.T) {
	factories, err := Factories()
	assert.NoError(t, err)
	receivers := factories.Receivers
	assert.Len(t, receivers, receiversCount)
	assert.NotNil(t, receivers["awscontainerinsightreceiver"])
	assert.NotNil(t, receivers["awsxray"])
	assert.NotNil(t, receivers["otlp"])
	assert.NotNil(t, receivers["tcplog"])
	assert.NotNil(t, receivers["udplog"])

	processors := factories.Processors
	assert.Len(t, processors, processorCount)
	assert.NotNil(t, processors["awsapplicationsignals"])
	assert.NotNil(t, processors["batch"])
	assert.NotNil(t, processors["cumulativetodelta"])
	assert.NotNil(t, processors["ec2tagger"])
	assert.NotNil(t, processors["metricstransform"])
	assert.NotNil(t, processors["transform"])
	assert.NotNil(t, processors["gpuattributes"])

	exporters := factories.Exporters
	assert.Len(t, exporters, exportersCount)
	assert.NotNil(t, exporters["awscloudwatchlogs"])
	assert.NotNil(t, exporters["awsemf"])
	assert.NotNil(t, exporters["awsxray"])
	assert.NotNil(t, exporters["awscloudwatch"])
	assert.NotNil(t, exporters["logging"])

	extensions := factories.Extensions
	assert.Len(t, extensions, extensionsCount)
	assert.NotNil(t, extensions["agenthealth"])
	assert.NotNil(t, extensions["awsproxy"])
}
