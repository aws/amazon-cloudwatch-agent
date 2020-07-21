// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package parsers

import (
	"github.com/aws/amazon-cloudwatch-agent/plugins/parsers/awscsm"
	"github.com/aws/amazon-cloudwatch-agent/plugins/parsers/emf"
	"github.com/influxdata/telegraf/plugins/parsers"
)

func NewEMFParser(config *parsers.Config) (parsers.Parser, error) {
	return &emf.EMFParser{
		MetricName:  config.MetricName,
		DefaultTags: config.DefaultTags,
	}, nil
}

func NewCSMParser(config *parsers.Config) (parsers.Parser, error) {
	parser := &awscsm.JSONParser{
		MetricName: "awscsm",
	}
	return parser, nil
}

func init() {
	parsers.RegisterParser("emf", NewEMFParser)
	parsers.RegisterParser("aws_csm", NewCSMParser)
}
