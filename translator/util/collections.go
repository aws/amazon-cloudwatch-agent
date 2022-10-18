// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

// CopyMap returns a new map that makes a shallow copy of all the
// references in the input map.
func CopyMap[K comparable, V any](m map[K]V) map[K]V {
	dupe := make(map[K]V)
	for k, v := range m {
		dupe[k] = v
	}
	return dupe
}

// MergeMaps merges multiple maps into a new one. Duplicate keys
// will take the last map's value.
func MergeMaps[K comparable, V any](maps ...map[K]V) map[K]V {
	merged := make(map[K]V)
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

// Pair is a struct with a K key and V value.
type Pair[K any, V any] struct {
	Key   K
	Value V
}

// NewPair creates a new Pair with key and value.
func NewPair[K any, V any](key K, value V) *Pair[K, V] {
	return &Pair[K, V]{key, value}
}

// Set is a map with a comparable K key and no
// meaningful value.
type Set[K comparable] map[K]any

// Add keys to the Set.
func (s Set[K]) Add(keys ...K) {
	for _, key := range keys {
		s[key] = nil
	}
}

// Remove a key from the Set.
func (s Set[K]) Remove(key K) {
	delete(s, key)
}

// Contains whether the key is in the Set.
func (s Set[K]) Contains(key K) bool {
	_, ok := s[key]
	return ok
}

// Keys creates a slice of the keys.
func (s Set[K]) Keys() []K {
	keys := make([]K, 0, len(s))
	for key := range s {
		keys = append(keys, key)
	}
	return keys
}

// NewSet creates a new Set with the keys provided.
func NewSet[K comparable](keys ...K) Set[K] {
	s := make(Set[K], len(keys))
	s.Add(keys...)
	return s
}
