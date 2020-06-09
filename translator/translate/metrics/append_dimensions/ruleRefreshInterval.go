package append_dimensions

import (
	"fmt"
)

type RefreshInterval struct {
}

func (r *RefreshInterval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = "refresh_interval_seconds"
	returnVal = fmt.Sprintf("%ds", 0)
	return
}

func init() {
	r := new(RefreshInterval)
	RegisterRule("refresh_interval", r)
}
