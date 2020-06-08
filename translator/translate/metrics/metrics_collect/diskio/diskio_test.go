package diskio

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiskIO(t *testing.T) {
	d := new(DiskIO)
	var input interface{}
	e := json.Unmarshal([]byte(`{"diskio": {
					"resources": [
						"sda"
					],
					"measurement": [
						"reads",
						"writes",
						"read_time",
						"write_time",
						"io_time"
					],
					"metrics_collection_interval": 60
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"devices":   []interface{}{"sda"},
			"fieldpass": []string{"reads", "writes", "read_time", "write_time", "io_time"},
			"interval":  "60s",
			"tags":      map[string]interface{}{"report_deltas": "true"},
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}

func TestDiskIOWithIOInProgress(t *testing.T) {
	d := new(DiskIO)
	var input interface{}
	e := json.Unmarshal([]byte(`{"diskio": {
					"resources": [
						"sda"
					],
					"measurement": [
						"reads",
						"writes",
						"read_time",
						"write_time",
						"io_time",
						"iops_in_progress"
					],
					"metrics_collection_interval": 60
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"devices":   []interface{}{"sda"},
			"fieldpass": []string{"reads", "writes", "read_time", "write_time", "io_time", "iops_in_progress"},
			"interval":  "60s",
			"tags":      map[string]interface{}{"report_deltas": "true", "ignored_fields_for_delta": "iops_in_progress"},
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}

func TestDiskIOWithReportDeltaTrue(t *testing.T) {
	d := new(DiskIO)
	var input interface{}
	e := json.Unmarshal([]byte(`{"diskio": {
					"resources": [
						"sda"
					],
					"measurement": [
						"reads",
						"writes",
						"read_time",
						"write_time",
						"io_time"
					],
					"metrics_collection_interval": 60,
					"report_deltas": true
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"devices":   []interface{}{"sda"},
			"fieldpass": []string{"reads", "writes", "read_time", "write_time", "io_time"},
			"interval":  "60s",
			"tags":      map[string]interface{}{"report_deltas": "true"},
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}

func TestDiskIOWithReportDeltaFalse(t *testing.T) {
	d := new(DiskIO)
	var input interface{}
	e := json.Unmarshal([]byte(`{"diskio": {
					"resources": [
						"sda"
					],
					"measurement": [
						"reads",
						"writes",
						"read_time",
						"write_time",
						"io_time"
					],
					"metrics_collection_interval": 60,
					"report_deltas": false
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"devices":   []interface{}{"sda"},
			"fieldpass": []string{"reads", "writes", "read_time", "write_time", "io_time"},
			"interval":  "60s",
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}
