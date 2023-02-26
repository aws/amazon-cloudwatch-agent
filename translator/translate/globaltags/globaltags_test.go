// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package globaltags

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalTags(t *testing.T) {
	g := new(GlobalTags)
	var input interface{}
	err := json.Unmarshal([]byte(`{"global_tags":{"dc":"us-east-3"}}`), &input)
	if err == nil {
		_, res := g.ApplyRule(input)
		globaltags := map[string]interface{}{
			"dc": "us-east-3",
		}
		assert.Equal(t, globaltags, res, "Expected to be equal")
	}
}
