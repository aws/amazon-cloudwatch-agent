package globaltags

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGlobalTags(t *testing.T) {
	g := new(GlobalTags)
	var input interface{}
	e := json.Unmarshal([]byte(`{"global_tags":{"dc":"us-east-3"}}`), &input)
	if e == nil {
		_, res := g.ApplyRule(input)
		globaltags := map[string]interface{}{
			"dc": "us-east-3",
		}
		assert.Equal(t, globaltags, res, "Expected to be equal")
	}
}
