// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collections

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

// GetOrDefault retrieves the value for the key in the map if it exists.
// If it doesn't exist, then returns the default value.
func GetOrDefault[K comparable, V any](m map[K]V, key K, defaultValue V) V {
	if value, ok := m[key]; ok {
		return value
	}
	return defaultValue
}

// MapSlice converts a slice of type K into a slice of type V
// using the provided mapper function.
func MapSlice[K any, V any](base []K, mapper func(K) V) []V {
	s := make([]V, len(base))
	for i, entry := range base {
		s[i] = mapper(entry)
	}
	return s
}

// WithNewKeys re-maps every key in a map as dictated by the mapper.
// If the key does not have an entry in the mapper, it is left untouched.
func WithNewKeys[K comparable, V any](base map[K]V, keyMapper map[K]K) map[K]V {
	mapped := make(map[K]V, len(base))
	for k, v := range base {
		if _, ok := keyMapper[k]; ok {
			mapped[keyMapper[k]] = v
		} else {
			mapped[k] = v
		}
	}
	return mapped
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

// ContainsAll whether the other set is a subset.
func (s Set[K]) ContainsAll(other Set[K]) bool {
	for key := range other {
		if !s.Contains(key) {
			return false
		}
	}
	return true
}

// Equal whether the two sets are the same.
func (s Set[K]) Equal(other Set[K]) bool {
	if len(s) != len(other) {
		return false
	}
	return s.ContainsAll(other)
}

// NewSet creates a new Set with the keys provided.
func NewSet[K comparable](keys ...K) Set[K] {
	s := make(Set[K], len(keys))
	s.Add(keys...)
	return s
}

// Range evaluates a function against each element in the slice.
func Range[T any](values []T, fn func(T) bool) bool {
	for _, value := range values {
		if !fn(value) {
			return false
		}
	}
	return true
}
