package emf

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEMF_HappyCase(t *testing.T) {
	obj := new(EMF)
	var input interface{}
	err := json.Unmarshal([]byte(`{"emf": {
					"service_address": "udp://:12345"
					}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"service_address": "udp://:12345",
			"data_format":     "emf",
			"name_override":   "emf",
		},
	}

	assert.Equal(t, expect, actual)
}

func TestEMF_MinimumConfig(t *testing.T) {
	obj := new(EMF)
	var input interface{}
	err := json.Unmarshal([]byte(`{"emf": {}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"service_address": "udp://127.0.0.1:25888",
			"data_format":     "emf",
			"name_override":   "emf",
		},
		map[string]interface{}{
			"service_address": "tcp://127.0.0.1:25888",
			"data_format":     "emf",
			"name_override":   "emf",
		},
	}

	assert.Equal(t, expect, actual)
}
