package mergeJsonRule

type MergeRule interface {
	Merge(source map[string]interface{}, result map[string]interface{})
}
