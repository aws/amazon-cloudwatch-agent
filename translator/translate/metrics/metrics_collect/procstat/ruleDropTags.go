package procstat

const (
	tagExcludeKey = "tagexclude"
)

var (
	tagExcludeValues = []string{"user"}
)

type DropTags struct {
}

func (i *DropTags) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = tagExcludeKey, tagExcludeValues
	return
}

func init() {
	RegisterRule("dropTags", new(DropTags))
}
