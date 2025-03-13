package nvme

import (
	"errors"
	"strconv"
	"strings"
)

const (
	nvmeDevicePrefix = "nvme"
)

type NvmeDeviceFileAttributes struct {
	controller int
	namespace  int
	partition  int
}

type Attribute interface {
	apply(*NvmeDeviceFileAttributes)
}

type nvmeDeviceAttributeFunc func(*NvmeDeviceFileAttributes)

func (f nvmeDeviceAttributeFunc) apply(e *NvmeDeviceFileAttributes) {
	f(e)
}

// TODO: if the format is invalid, we shouldn't silently fail
// TODO: also this seems overly complicated. maybe simplify. evaluate just using a regex instead
func ParseNvmeDeviceFileName(device string) (NvmeDeviceFileAttributes, error) {
	if !strings.HasPrefix(device, nvmeDevicePrefix) {
		return NvmeDeviceFileAttributes{
			controller: -1,
			namespace:  -1,
			partition:  -1,
		}, errors.New("device is not prefixed with nvme")
	}

	trimmed := strings.TrimPrefix(device, nvmeDevicePrefix)

	controllerEndIdx := strings.Index(trimmed, "n")
	if controllerEndIdx == -1 {
		controllerEndIdx = len(trimmed)
		return newNvmeDeviceFileAttributes(
			withController(substring(trimmed, 0, controllerEndIdx)),
		), nil
	}

	namespaceEndIdx := strings.Index(trimmed, "p")
	if namespaceEndIdx == -1 {
		namespaceEndIdx = len(trimmed)
		return newNvmeDeviceFileAttributes(
			withController(substring(trimmed, 0, controllerEndIdx)),
			withNamespace(substring(trimmed, controllerEndIdx+1, namespaceEndIdx)),
		), nil
	}

	return newNvmeDeviceFileAttributes(
		withController(substring(trimmed, 0, controllerEndIdx)),
		withNamespace(substring(trimmed, controllerEndIdx+1, namespaceEndIdx)),
		withPartition(substring(trimmed, namespaceEndIdx+1, len(trimmed))),
	), nil
}

func (n *NvmeDeviceFileAttributes) Controller() int {
	return n.controller
}

func (n *NvmeDeviceFileAttributes) Namespace() int {
	return n.namespace
}

func (n *NvmeDeviceFileAttributes) Partition() int {
	return n.partition
}

func newNvmeDeviceFileAttributes(attributes ...Attribute) NvmeDeviceFileAttributes {
	n := &NvmeDeviceFileAttributes{
        controller: -1,
        namespace:  -1,
        partition:  -1,
    }
	for _, attribute := range attributes {
		attribute.apply(n)
	}
	return *n
}

func withController(controller string) Attribute {
	c := atoiOrNegative(controller)
	return nvmeDeviceAttributeFunc(func(attr *NvmeDeviceFileAttributes) {
		attr.controller = c
	})
}

func withNamespace(namespace string) Attribute {
	n := atoiOrNegative(namespace)
	return nvmeDeviceAttributeFunc(func(attr *NvmeDeviceFileAttributes) {
		attr.namespace = n
	})
}

func withPartition(partition string) Attribute {
	p := atoiOrNegative(partition)
	return nvmeDeviceAttributeFunc(func(attr *NvmeDeviceFileAttributes) {
		attr.partition = p
	})
}

// TODO: evaluate if this is really needed with the new parsing logic
func substring(s string, l, r int) string {
	if l < 0 {
		return ""
	}
	if r > len(s) {
		return ""
	}
	if l >= r {
		return ""
	}

	return s[l:r]
}

// TODO: return error?
func atoiOrNegative(a string) int {
	i, err := strconv.Atoi(a)
	if err != nil {
		return -1
	}
	return i
}
