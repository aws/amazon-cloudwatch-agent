package metrics_collect

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectMetrics(t *testing.T) {
	c := new(CollectMetrics)
	var input interface{}
	e := json.Unmarshal([]byte(`{"metrics_collected":{}}`), &input)
	if e == nil {
		_, actual := c.ApplyRule(input)
		expected := map[string]interface{}(map[string]interface{}{})
		assert.Equal(t, expected, actual, "Expected to be equal")
	}
}
