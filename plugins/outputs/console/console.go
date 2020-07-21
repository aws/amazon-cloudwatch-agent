// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package console

import (
	"fmt"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
)

type Console struct{}

func (s *Console) Description() string {
	return "a demo output"
}

func (s *Console) SampleConfig() string {
	return ``
}

func (s *Console) Init() error {
	return nil
}

func (s *Console) Connect() error {
	return nil
}

func (s *Console) Close() error {
	return nil
}

func (s *Console) Write(metrics []telegraf.Metric) error {
	for _, metric := range metrics {
		fmt.Println(metric)
	}
	return nil
}

func init() {
	outputs.Add("console", func() telegraf.Output { return &Console{} })
}
