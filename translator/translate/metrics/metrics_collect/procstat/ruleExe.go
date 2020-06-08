package procstat

type Exe struct{}

const keyExe = "exe"

func (t *Exe) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m[keyExe]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = keyExe
		returnVal = m[keyExe]
	}
	return
}

func init() {
	e := new(Exe)
	RegisterRule(keyExe, e)
}
