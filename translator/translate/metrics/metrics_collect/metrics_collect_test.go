// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics_collect

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectMetrics(t *testing.T) {
	c := new(CollectMetrics)
	var input interface{}
	err := json.Unmarshal([]byte(`{"metrics_collected":{}}`), &input)
	require.Nil(t, err)
	_, actual := c.ApplyRule(input)
	expected := map[string]interface{}{}
	assert.Equal(t, expected, actual)
}
