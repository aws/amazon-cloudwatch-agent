package metric_decoration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

//Check the case when the input is in "cpu":{//specific configuration}
func TestMetricDecoration_ApplyRule(t *testing.T) {
	c := new(MetricDecoration)
	//Check whether override default config
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"metrics_collected": {
				"cpu": {
					"measurement": [
						{"name": "cpu_usage_idle", "rename": "CPU", "unit": "unit"},
						{"name": "cpu_usage_nice", "unit": "unit"},
						"cpu_usage_guest"
					]
				}
			}}`), &input)

	if e == nil {
		_, val := c.ApplyRule(input)
		expected := []interface{}{
			map[string]string{
				"rename":   "CPU",
				"unit":     "unit",
				"category": "cpu",
				"name":     "cpu_usage_idle",
			},
			map[string]string{
				"category": "cpu",
				"name":     "cpu_usage_nice",
				"unit":     "unit",
			},
		}
		assert.Equal(t, expected, val, "Expect to be equal")
	} else {
		panic(e)
	}
}
