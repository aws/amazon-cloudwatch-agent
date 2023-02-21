// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package parsers

import (
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/parsers"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/parsers/emf"
)

func EMFParserCreator(metricName string) telegraf.Parser {
	// Telegraf will create parser with metricName only, and initFromConfig will be called to get the tags
	return &emf.EMFParser{
		MetricName: metricName,
	}
}

func init() {
	parsers.Add("emf", EMFParserCreator)
}
