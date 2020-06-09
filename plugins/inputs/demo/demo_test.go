package demo

import (
	"testing"

	"github.com/influxdata/telegraf/testutil"
)

func TestDemo(t *testing.T) {
	s := &Demo{
		Amplitude: 10.0,
	}

	for i := 0.0; i < 10.0; i++ {
		var acc testutil.Accumulator
		s.Gather(&acc)
	}
}
