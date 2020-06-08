package procstat

type PidFile struct{}

const keyPidFile = "pid_file"

func (p *PidFile) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if _, ok := m[keyPidFile]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		returnKey = keyPidFile
		returnVal = m[keyPidFile]
	}
	return
}

func init() {
	p := new(PidFile)
	RegisterRule(keyPidFile, p)
}
