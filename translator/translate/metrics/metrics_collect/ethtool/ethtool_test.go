package ethtool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	d := new(Ethtool)
	var input interface{}
	e := json.Unmarshal([]byte(`{"ethtool": {
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"interface_include": []string{"*"},
			"fieldpass":         []string{},
		},
		}
		assert.Equal(t, d, actual, "Expected to be equal")
	}
}

func TestFullConfig(t *testing.T) {
	d := new(Ethtool)
	var input interface{}
	e := json.Unmarshal([]byte(`{"ethtool": {
					"interface_include": [
						"eth0"
					],
					"interface_exclude": [
						"eth1"
					],
					"metrics_include": [
						"bw_in_allowance_exceeded",
					],
					}}`), &input)
	if e == nil {
		_, actual := d.ApplyRule(input)

		d := []interface{}{map[string]interface{}{
			"interface_include": []string{"eth0"},
			"interface_exclude": []string{"eth1"},
			"fieldpass":         []string{"bw_in_allowance_exceeded"},
		},
		}

		assert.Equal(t, d, actual, "Expected to be equal")
	}
}
