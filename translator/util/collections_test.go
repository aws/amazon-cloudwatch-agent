package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopyMapHasSameValues(t *testing.T) {
	m := map[string]interface{}{
		"foo": 1,
		"bar": 2,
		"baz": 3,
	}
	copied := CopyMap(m)
	assertMapsEqual(t, m, copied)
}

func TestCopyMapDoesNotShareReferenceToOriginalMap(t *testing.T) {
	m := map[string]interface{}{
		"foo": 1,
		"bar": 2,
		"baz": 3,
	}
	copied := CopyMap(m)
	assertMapsEqual(t, m, copied)
	delete(m, "foo")
	_, ok := m["foo"]
	assert.False(t, ok)

	val, ok := copied["foo"]
	assert.True(t, ok)
	assert.Equal(t, 1, val)
}

// TODO: could change the implementation to recurse and do a deep copy of everything in the
//       input map, but not necessary at the moment. Documenting current behavior here.
func TestCopyMapKeepsShallowReferenceToValuesInMap(t *testing.T) {
	m := map[string]interface{}{
		"foo": 1,
		"bar": 2,
		"baz": map[string]int{"baz": 3, "foo": 1},
	}
	copied := CopyMap(m)
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

func assertMapsEqual(t *testing.T, m1, m2 map[string]interface{}) {
	t.Helper()

	assert.Equal(t, len(m1), len(m2))

	for k, expected := range m1 {
		actual, ok := m2[k]
		assert.True(t, ok)
		assert.Equal(t, expected, actual)
	}
}
