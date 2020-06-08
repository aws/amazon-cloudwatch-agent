package config

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestGetJsonSchema(t *testing.T) {
	jsonFile, err := ioutil.ReadFile("./schema.json")
	if err != nil {
		panic(err)
	}
	assert.Equal(t, string(jsonFile), GetJsonSchema(), "Json schema is inconsistent")
}

func TestGetFormattedPath(t *testing.T) {
	assert.Equal(t, "/metrics/metrics_collected/cpu/resources/1", GetFormattedPath("(root).metrics.metrics_collected.cpu.resources.1"))
	assert.Equal(t, "/metrics/metrics_collected/cpu", GetFormattedPath("(root).metrics.metrics_collected.cpu"))
}
