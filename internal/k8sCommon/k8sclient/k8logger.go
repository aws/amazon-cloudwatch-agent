package k8sclient

import "log"

type K8Logger struct {
}

// Log implements Logger by calling f(keyvals...).
func (f K8Logger) Log(keyvals ...interface{}) error {
	log.Printf("k8 log : %v", keyvals)
	return nil
}
