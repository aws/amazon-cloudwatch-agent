// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mapWithExpiry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapWithExpiry_add(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Set("key1", "value1")
	val, ok := store.Get("key1")
	assert.Equal(t, true, ok)
	assert.Equal(t, "value1", val.(string))

	val, ok = store.Get("key2")
	assert.Equal(t, false, ok)
	assert.Equal(t, nil, val)
}

func TestMapWithExpiry_delete(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Set("key1", "value1")
	val, ok := store.Get("key1")
	assert.Equal(t, true, ok)
	assert.Equal(t, "value1", val.(string))

	store.Delete("key1")
	val, ok = store.Get("key1")
	assert.Equal(t, false, ok)
	assert.Equal(t, nil, val)
}

func TestMapWithExpiry_cleanup(t *testing.T) {
	store := NewMapWithExpiry(50 * time.Millisecond)
	store.Set("key1", "value1")

	store.CleanUp(time.Now())
	val, ok := store.Get("key1")
	assert.Equal(t, true, ok)
	assert.Equal(t, "value1", val.(string))
	assert.Equal(t, 1, store.Size())

	require.Eventually(t, func() bool {
		store.CleanUp(time.Now())
		return store.Size() == 0
	}, time.Second, 50*time.Millisecond)
	val, ok = store.Get("key1")
	assert.Equal(t, false, ok)
	assert.Equal(t, nil, val)
}
