// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collections

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

func TestCopyMapHasSameValues(t *testing.T) {
	m := map[string]interface{}{
		"foo": 1,
		"bar": 2,
		"baz": 3,
	}
	copied := maps.Clone(m)
	assertMapsEqual(t, m, copied)
}

func TestCopyMapDoesNotShareReferenceToOriginalMap(t *testing.T) {
	m := map[string]interface{}{
		"foo": 1,
		"bar": 2,
		"baz": 3,
	}
	copied := maps.Clone(m)
	assertMapsEqual(t, m, copied)
	delete(m, "foo")
	_, ok := m["foo"]
	assert.False(t, ok)

	val, ok := copied["foo"]
	assert.True(t, ok)
	assert.Equal(t, 1, val)
}

// TODO: could change the implementation to recurse and do a deep copy of everything in the
// input map, but not necessary at the moment. Documenting current behavior here.
func TestCopyMapKeepsShallowReferenceToValuesInMap(t *testing.T) {
	m := map[string]interface{}{
		"foo": 1,
		"bar": 2,
		"baz": map[string]int{"baz": 3, "foo": 1},
	}
	copied := maps.Clone(m)
	assertMapsEqual(t, m, copied)

	baz, ok := m["baz"]
	assert.True(t, ok)
	bazMap, ok := baz.(map[string]int)
	assert.True(t, ok)

	copiedBaz, ok := copied["baz"]
	assert.True(t, ok)
	copiedBazMap, ok := copiedBaz.(map[string]int)
	assert.True(t, ok)
	_, ok = copiedBazMap["baz"]
	assert.True(t, ok)

	// delete from original map
	delete(bazMap, "baz")
	_, ok = bazMap["baz"]
	assert.False(t, ok)
	// deleting from the original map reference also removes it from the copy
	_, ok = copiedBazMap["baz"]
	assert.False(t, ok)
}

func TestMergeMaps(t *testing.T) {
	m1 := map[string]int{"first": 1, "overwrite": 1}
	m2 := map[string]int{"second": 2, "overwrite": 2}
	got := MergeMaps(m1, m2)
	require.Len(t, got, 3)
	value, ok := got["overwrite"]
	require.True(t, ok)
	require.Equal(t, 2, value)
}

func TestGetOrDefault(t *testing.T) {
	m1 := map[string]int{"first": 1, "second": 2}
	got := GetOrDefault(m1, "first", 0)
	require.Equal(t, 1, got)
	got = GetOrDefault(m1, "missing", 0)
	require.Equal(t, 0, got)
}

func TestKeys(t *testing.T) {
	m1 := map[string]int{"first": 1, "second": 2}
	got := maps.Keys(m1)
	sort.Strings(got)
	require.Equal(t, []string{"first", "second"}, got)
}

func TestValues(t *testing.T) {
	m1 := map[string]int{"first": 1, "second": 2}
	got := maps.Values(m1)
	sort.Ints(got)
	require.Equal(t, []int{1, 2}, got)
}

func TestMapSlice(t *testing.T) {
	s := []string{"test", "value"}
	got := MapSlice(s, strings.ToUpper)
	require.Equal(t, []string{"TEST", "VALUE"}, got)
}

func TestWithNewKeys(t *testing.T) {
	base := map[string]int{
		"one": 100,
		"two": 500,
	}
	mapper := map[string]string{
		"two": "five",
	}

	got := WithNewKeys(base, mapper)
	require.Equal(t, map[string]int{
		"one":  100,
		"five": 500,
	}, got)
}

func TestPair(t *testing.T) {
	pair := NewPair("key", "value")
	require.Equal(t, "key", pair.Key)
	require.Equal(t, "value", pair.Value)
}

func TestSet(t *testing.T) {
	set := NewSet(1, 2)
	require.True(t, set.Contains(1))
	set.Remove(1)
	require.False(t, set.Contains(1))
	require.Equal(t, []int{2}, maps.Keys(set))
}

func assertMapsEqual(t *testing.T, m1, m2 map[string]interface{}) {
	t.Helper()

	assert.Equal(t, len(m1), len(m2))

	for k, expected := range m1 {
		actual, ok := m2[k]
		assert.True(t, ok)
		assert.Equal(t, expected, actual)
	}
}
