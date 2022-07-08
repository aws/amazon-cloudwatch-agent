// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package parsers

import (
	"github.com/aws/amazon-cloudwatch-agent/plugins/parsers/awscsm"
	"github.com/aws/amazon-cloudwatch-agent/plugins/parsers/emf"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/parsers"
)

func EMFParserCreator(metricName string) telegraf.Parser {
	// Telegraf will create parser with metricName only, and initFromConfig will be called to get the tags
	return &emf.EMFParser{
		MetricName: metricName,
	}
}

func CSMParserCreator(metricName string) telegraf.Parser {
	parser := &awscsm.JSONParser{
		MetricName: "awscsm",
	}
	return parser
}

func init() {
	parsers.Add("emf", EMFParserCreator)
	parsers.Add("aws_csm", CSMParserCreator)
}
