// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitMapKeys(t *testing.T) {
	testMap := make(map[string]*EC2MetaData)
	testMap["a"] = nil
	testMap["b"] = nil
	testMap["c"] = nil
	testMap["4"] = nil
	result := splitMapKeys(testMap, 2)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, 2, len(result[0]))
	assert.Equal(t, 2, len(result[1]))

	testMap["5"] = nil
	result2 := splitMapKeys(testMap, 9)
	assert.Equal(t, 1, len(result2))
	assert.Equal(t, 5, len(result2[0]))

	result3 := splitMapKeys(testMap, 3)
	assert.Equal(t, 2, len(result3))
	assert.Equal(t, 3, len(result3[0]))
	assert.Equal(t, 2, len(result3[1]))

}

func TestSplitMapKeys_Empty(t *testing.T) {
	testMap := make(map[string]*EC2MetaData)
	result := splitMapKeys(testMap, 2)
	assert.Equal(t, 0, len(result))
}

func TestSplitMapKeys_Panic(t *testing.T) {
	defer func() { recover() }()
	testMap := make(map[string]*EC2MetaData)
	testMap["a"] = nil
	testMap["b"] = nil
	splitMapKeys(testMap, 0)
	t.Errorf("should have panicked")
}
