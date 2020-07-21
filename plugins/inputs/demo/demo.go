// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package demo

import (
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

type Demo struct {
	Amplitude float64
}

var DemoConfig = `
  ## Set the amplitude
  amplitude = 10.0
`

func (s *Demo) SampleConfig() string {
	return DemoConfig
}

func (s *Demo) Description() string {
	return "Generate sawtooth and square wave metric for demonstration purposes"
}

func (s *Demo) Gather(acc telegraf.Accumulator) error {
	now := time.Now()
	amp := int64(s.Amplitude)
	if amp < 0 {
		amp = -amp
	}
	if amp < 1 {
		amp = 1
	}

	fields := make(map[string]interface{})
	fields["sawtooth"] = float64(now.Unix() % amp)

	if (now.Unix()/amp)%2 == 0 {
		fields["square"] = s.Amplitude
	} else {
		fields["square"] = -s.Amplitude
	}

	tags := make(map[string]string)

	acc.AddFields("demo", fields, tags)

	return nil
}

func init() {
	inputs.Add("demo", func() telegraf.Input { return &Demo{} })
}
