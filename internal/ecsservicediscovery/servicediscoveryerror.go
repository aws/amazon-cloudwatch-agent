package ecsservicediscovery

import "fmt"

type ServiceDiscoveryError struct {
	msg       string
	origError *error
}

func (p ServiceDiscoveryError) Error() string {
	if p.origError != nil {
		return fmt.Sprintf("%s; original error: %s", p.msg, (*p.origError).Error())
	}
	return p.msg
}

func newServiceDiscoveryError(errMsg string, origErr *error) error {
	return &ServiceDiscoveryError{
		msg:       errMsg,
		origError: origErr,
	}
}
