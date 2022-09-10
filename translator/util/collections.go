package util

// CopyMap returns a new map that makes a shallow copy of all the
// references in the input map.
func CopyMap(m map[string]interface{}) map[string]interface{} {
	dupe := make(map[string]interface{})
	for k, v := range m {
		dupe[k] = v
	}
	return dupe
}

func MergeMaps(maps ...map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}
