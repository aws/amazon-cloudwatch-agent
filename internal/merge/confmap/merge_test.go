// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package confmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeConflictError(t *testing.T) {
	mce := &MergeConflictError{
		conflicts: []mergeConflict{
			{section: "one", keys: []string{"two", "three"}},
			{section: "four", keys: []string{"five"}},
		},
	}
	assert.Equal(t, "merge conflict in one: [two three], four: [five]", mce.Error())
}
