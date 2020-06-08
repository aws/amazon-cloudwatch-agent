package awscsm

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"math/rand"
	"sort"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/providers"
)

// Samples represent the raw request received from the input.
type Samples struct {
	list []map[string]interface{}
	// events will be used to count sample event that contain the
	// same signature as other samples received.
	events map[uint32]int
}

func newSamples() Samples {
	return Samples{
		events: map[uint32]int{},
	}
}

// ShouldAdd will return true or false depending on whether or not
// we should store the sample. How a sample is chosen to be added is
// done by a randomization algorithm.
func (s Samples) ShouldAdd(threshold float64) bool {
	return rand.Float64() < threshold
}

// Add will add the sample to the sample list
func (s *Samples) Add(m map[string]interface{}) {
	s.list = append(s.list, m)
}

// Len will return the proper length of samples
func (s Samples) Len() int64 {
	return int64(len(s.list))
}

// Count will increase the events count for a given signature
func (s *Samples) Count(signature uint32) bool {
	v, ok := s.events[signature]
	v++

	s.events[signature] = v
	return ok
}

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

func getSampleSignature(definitions providers.Definitions, raw map[string]interface{}) uint32 {
	keys := []string{}
	for k := range raw {
		def, ok := definitions.Entries.Get(k)
		if !ok {
			continue
		}

		if def.KeyType.IsSample() {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)
	buf := bytes.Buffer{}
	for _, k := range keys {
		v := raw[k]
		buf.WriteString(fmt.Sprintf("%s:%v\n", k, v))
	}

	return crc32.Checksum(buf.Bytes(), crc32cTable)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
