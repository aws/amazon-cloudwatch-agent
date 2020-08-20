package migrate

func init() {
	AddRule(ProcStatRule)
}

//  [inputs]
//  [[inputs.procstat]]
//    fieldpass = ["cpu_usage", "memory_rss"]
//    pid_file = "/var/run/example1.pid"
//    pid_finder = "native"
//+   tagexclude = ["user", "result"]
func ProcStatRule(conf map[string]interface{}) error {
	ps := getItem(conf, "inputs", "procstat")

	for _, p := range ps {
		tagexclude, ok := p["tagexclude"].([]interface{})
		var newTagEx []interface{}
		if ok {
			newTagEx = append(newTagEx, tagexclude...)
		}
		newTagEx = append(newTagEx, "user", "result")

		p["tagexclude"] = newTagEx
	}

	return nil
}
