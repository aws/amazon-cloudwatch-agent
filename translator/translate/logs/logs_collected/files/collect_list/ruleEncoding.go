package collect_list

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/ianaindex"
)

const EncodingSectionKey = "encoding"

type Encoding struct {
}

func (e *Encoding) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultCase(EncodingSectionKey, "", input)
	if val == "" {
		return
	}
	if val, ok := val.(string); ok {
		if _, name := charset.Lookup(val); name == "" {
			if _, err := ianaindex.IANA.Encoding(val); err != nil {
				translator.AddErrorMessages(GetCurPath()+EncodingSectionKey, fmt.Sprintf("Encoding %s is an invalid value.", val))
				return
			}
		}
	} else {
		translator.AddErrorMessages(GetCurPath()+EncodingSectionKey, fmt.Sprintf("value for %s must be string", EncodingSectionKey))
		return
	}
	returnKey = key
	returnVal = val
	return
}

func init() {
	l := new(Encoding)
	r := []Rule{l}
	RegisterRule(EncodingSectionKey, r)
}
