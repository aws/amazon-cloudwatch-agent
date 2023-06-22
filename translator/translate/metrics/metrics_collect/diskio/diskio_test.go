// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

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
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}

func TestDiskIOWithIOInProgressWithDiskIOPrefix(t *testing.T) {
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
						"diskio_iops_in_progress"
					],
					"metrics_collection_interval": 60
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"devices":   []interface{}{"sda"},
			"fieldpass": []string{"reads", "writes", "read_time", "write_time", "io_time", "iops_in_progress"},
			"interval":  "60s",
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}

func TestDiskIOWithIOInProgressWithRename(t *testing.T) {
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
						{
                        "name": "iops_in_progress",
                        "rename": "DRIVER_DISKIO_IOPS_IN_PROGRESS",
                        "unit": "Count"
                    	}
					],
					"metrics_collection_interval": 60
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"devices":   []interface{}{"sda"},
			"fieldpass": []string{"reads", "writes", "read_time", "write_time", "io_time", "iops_in_progress"},
			"interval":  "60s",
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}

func TestDiskIOWithIOInProgressWithRenameAndDiskIOPrefix(t *testing.T) {
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
						{
                        "name": "diskio_iops_in_progress",
                        "rename": "DRIVER_DISKIO_IOPS_IN_PROGRESS",
                        "unit": "Count"
                    	}
					],
					"metrics_collection_interval": 60
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"devices":   []interface{}{"sda"},
			"fieldpass": []string{"reads", "writes", "read_time", "write_time", "io_time", "iops_in_progress"},
			"interval":  "60s",
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}
