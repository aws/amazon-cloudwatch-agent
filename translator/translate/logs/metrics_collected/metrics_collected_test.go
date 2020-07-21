// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics_collected

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectMetrics(t *testing.T) {
	c := new(CollectMetrics)
	var input interface{}
	err := json.Unmarshal([]byte(`{"metrics_collected":{}}`), &input)
	assert.NoError(t, err)
	_, actual := c.ApplyRule(input)
	expected := map[string]map[string]interface{}{"inputs": {}, "processors": {}}
	assert.Equal(t, expected, actual, "Expected to be equal")
}
