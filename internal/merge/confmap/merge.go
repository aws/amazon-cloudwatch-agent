// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package confmap

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/knadh/koanf/maps"
)

const (
	keyReceivers  = "receivers"
	keyProcessors = "processors"
	keyExporters  = "exporters"
	keyExtensions = "extensions"
	keyService    = "service"
	keyPipelines  = "pipelines"
)

var (
	// restrictedSections in the OTEL configuration that cannot have duplicate keys
	restrictedSections = [][]string{
		{keyReceivers},
		{keyProcessors},
		{keyExporters},
		{keyExtensions},
		{keyService, keyPipelines},
	}
)

type mergeConflict struct {
	// section where conflict occurs
	section string
	// keys in the section that have conflicts
	keys []string
}

type MergeConflictError struct {
	conflicts []mergeConflict
}

func (e *MergeConflictError) Error() string {
	var conflictStrs []string
	for _, conflict := range e.conflicts {
		conflictStrs = append(conflictStrs, fmt.Sprintf("%s: %s", conflict.section, conflict.keys))
	}
	return fmt.Sprintf("merge conflict in %s", strings.Join(conflictStrs, ", "))
}

// mergeMaps checks for conflicts and merges the service before merging the rest of the maps.
func mergeMaps(src, dest map[string]any) error {
	mce := &MergeConflictError{}
	for _, section := range restrictedSections {
		if mc := checkConflicts(src, dest, section); mc != nil {
			mce.conflicts = append(mce.conflicts, *mc)
		}
	}
	if len(mce.conflicts) > 0 {
		return mce
	}
	mergeServices(src, dest)
	maps.Merge(src, dest)
	return nil
}

// checkConflicts for overlapping keys in the maps at the path.
func checkConflicts(src, dest map[string]any, path []string) *mergeConflict {
	srcMap, srcOK := getMapValue[map[string]any](src, path)
	destMap, destOK := getMapValue[map[string]any](dest, path)
	if !srcOK || !destOK {
		return nil
	}
	var keys []string
	for key := range destMap {
		if _, ok := srcMap[key]; ok && !reflect.DeepEqual(srcMap[key], destMap[key]) {
			keys = append(keys, key)
		}
	}
	if len(keys) > 0 {
		return &mergeConflict{section: strings.Join(path, KeyDelimiter), keys: keys}
	}
	return nil
}

// mergeServices overwrites the source service::extensions with the merged results. This is because the default
// maps.Merge just sets the destination to the source for slices.
func mergeServices(src, dest map[string]any) {
	srcMap, srcOK := getMapValue[map[string]any](src, []string{keyService})
	destMap, destOK := getMapValue[map[string]any](dest, []string{keyService})
	if !srcOK || !destOK {
		return
	}
	results := mergeSlices(srcMap[keyExtensions], destMap[keyExtensions])
	if results != nil {
		srcMap[keyExtensions] = results
	}
}

// mergeSlices appends the deduplicated items in the destination to the source slice.
func mergeSlices(src, dest any) any {
	if src == nil || dest == nil {
		return nil
	}

	srcVal := reflect.ValueOf(src)
	destVal := reflect.ValueOf(dest)

	if srcVal.Kind() != reflect.Slice || destVal.Kind() != reflect.Slice {
		return nil
	}

	result := reflect.MakeSlice(srcVal.Type(), 0, srcVal.Len()+destVal.Len())
	for i := 0; i < srcVal.Len(); i++ {
		result = reflect.Append(result, srcVal.Index(i))
	}

	for i := 0; i < destVal.Len(); i++ {
		item := destVal.Index(i)
		if !containsInSlice(result, item) {
			result = reflect.Append(result, item)
		}
	}
	return result.Interface()
}

func containsInSlice(slice, item reflect.Value) bool {
	if slice.Kind() != reflect.Slice {
		return false
	}
	for i := 0; i < slice.Len(); i++ {
		if slice.Index(i).Equal(item) {
			return true
		}
	}
	return false
}

// getMapValue uses maps.Search to find the value at the path and casts it.
func getMapValue[T any](m map[string]any, path []string) (T, bool) {
	var zeroValue T
	found := maps.Search(m, path)
	if found == nil {
		return zeroValue, false
	}
	cast, ok := found.(T)
	if !ok {
		return zeroValue, false
	}
	return cast, true
}
