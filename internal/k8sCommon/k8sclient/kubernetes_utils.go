// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	// kubeAllowedStringAlphaNums holds the characters allowed in replicaset names from as parent deployment
	// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apimachinery/pkg/util/rand/rand.go#L121
	kubeAllowedStringAlphaNums = "bcdfghjklmnpqrstvwxz2456789"
)

var (
	// ReplicaSet name = Deployment name + "-" + up to 10 alphanumeric characters string, if the ReplicaSet was created through a deployment
	// The suffix string of the ReplicaSet name is an int32 number (0 to 4,294,967,295) that is cast to a string and then
	// mapped to an alphanumeric value with only the following characters allowed: "bcdfghjklmnpqrstvwxz2456789".
	// The suffix string length is therefore nondeterministic. The regex accepts a suffix of length 6-10 to account for
	// ReplicaSets not managed by deployments that may have similar names.
	// Suffix Generation: https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/controller_utils.go#L1201
	// Alphanumeric Mapping: https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apimachinery/pkg/util/rand/rand.go#L121)
	replicaSetWithDeploymentNamePattern = fmt.Sprintf(`^(.+)-[%s]{6,10}$`, kubeAllowedStringAlphaNums)
	deploymentFromReplicaSetPattern     = regexp.MustCompile(replicaSetWithDeploymentNamePattern)
	// if a pod is launched directly by a replicaSet or daemonSet (with a given name by users), its name has the following pattern:
	// Pod name = ReplicaSet name + 5 alphanumeric characters long string
	// some code reference for daemon set:
	// 1. daemonset uses the strategy to create pods: https://github.com/kubernetes/kubernetes/blob/82e3a671e79d1740ab9a3b3fac8a3bb7d065a6fb/pkg/registry/apps/daemonset/strategy.go#L46
	// 2. the strategy uses SimpleNameGenerator to create names: https://github.com/kubernetes/kubernetes/blob/82e3a671e79d1740ab9a3b3fac8a3bb7d065a6fb/staging/src/k8s.io/apiserver/pkg/storage/names/generate.go#L53
	// 3. the random name generator only use non vowels char + numbers: https://github.com/kubernetes/kubernetes/blob/82e3a671e79d1740ab9a3b3fac8a3bb7d065a6fb/staging/src/k8s.io/apimachinery/pkg/util/rand/rand.go#L83
	podWithSuffixPattern                = fmt.Sprintf(`^(.+)-[%s]{5}$`, kubeAllowedStringAlphaNums)
	replicaSetOrDaemonSetFromPodPattern = regexp.MustCompile(podWithSuffixPattern)

	// Pattern for StatefulSet: <statefulset-name>-<ordinal>
	reStatefulSet = regexp.MustCompile(`^(.+)-(\d+)$`)
)

func attachNamespace(resourceName, namespace string) string {
	// character "@" is not allowed in kubernetes resource names: https://unofficial-kubernetes.readthedocs.io/en/latest/concepts/overview/working-with-objects/names/
	return resourceName + "@" + namespace
}

func getServiceAndNamespace(service *corev1.Service) string {
	return attachNamespace(service.Name, service.Namespace)
}

func ExtractResourceAndNamespace(serviceOrWorkloadAndNamespace string) (string, string) {
	// extract service name and namespace from serviceAndNamespace
	parts := strings.Split(serviceOrWorkloadAndNamespace, "@")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func extractWorkloadNameFromRS(replicaSetName string) (string, error) {
	match := deploymentFromReplicaSetPattern.FindStringSubmatch(replicaSetName)
	if match != nil {
		return match[1], nil
	}

	return "", errors.New("failed to extract workload name from replicatSet name: " + replicaSetName)
}

func extractWorkloadNameFromPodName(podName string) (string, error) {
	match := replicaSetOrDaemonSetFromPodPattern.FindStringSubmatch(podName)
	if match != nil {
		return match[1], nil
	}

	return "", errors.New("failed to extract workload name from pod name: " + podName)
}

func GetWorkloadAndNamespace(pod *corev1.Pod) string {
	var workloadAndNamespace string
	if pod.OwnerReferences != nil {
		for _, ownerRef := range pod.OwnerReferences {
			if workloadAndNamespace != "" {
				break
			}

			switch ownerRef.Kind {
			case "ReplicaSet":
				if workloadName, err := extractWorkloadNameFromRS(ownerRef.Name); err == nil {
					// when the replicaSet is created by a deployment, use deployment name
					workloadAndNamespace = attachNamespace(workloadName, pod.Namespace)
				} else if workloadName, err := extractWorkloadNameFromPodName(pod.Name); err == nil {
					// when the replicaSet is not created by a deployment, use replicaSet name directly
					workloadAndNamespace = attachNamespace(workloadName, pod.Namespace)
				}
			case "StatefulSet":
				workloadAndNamespace = attachNamespace(ownerRef.Name, pod.Namespace)
			case "DaemonSet":
				workloadAndNamespace = attachNamespace(ownerRef.Name, pod.Namespace)
			}
		}
	}

	return workloadAndNamespace
}

// InferWorkloadName tries to parse the given podName to find the top-level workload name.
//
// 1) If it matches <statefulset>-<ordinal>, return <statefulset>.
// 2) If it matches <something>-<5charSuffix>:
//   - If <something> is <deployment>-<6–10charSuffix>, return <deployment>.
//   - Else return <something> (likely a bare ReplicaSet or DaemonSet).
//
// 3) If no pattern matches, return the original podName.
//
// Caveat: You can't reliably distinguish DaemonSet vs. bare ReplicaSet by name alone.
// In some edge cases when the deployment name is longer than 47 characters, The regex pattern is
// not reliable. See reference:
//   - https://pauldally.medium.com/why-you-try-to-keep-your-deployment-names-to-47-characters-or-less-1f93a848d34c
//   - https://github.com/kubernetes/kubernetes/issues/116447#issuecomment-1530652258
//
// For that, we fall back to use service name as last defense.
func InferWorkloadName(podName, fallbackServiceName string) string {
	// 1) Check if it's a StatefulSet pod: <stsName>-<ordinal>
	if matches := reStatefulSet.FindStringSubmatch(podName); matches != nil {
		return matches[1] // e.g. "mysql-0" => "mysql"
	}

	// 2) Check if it's a Pod with a 5-char random suffix: <parentName>-<5Chars>
	if matches := replicaSetOrDaemonSetFromPodPattern.FindStringSubmatch(podName); matches != nil {
		parentName := matches[1]

		// If parentName ends with 6–10 random chars, that parent is a Deployment-based ReplicaSet.
		// So the top-level workload is the first part before that suffix.
		if rsMatches := deploymentFromReplicaSetPattern.FindStringSubmatch(parentName); rsMatches != nil {
			return rsMatches[1] // e.g. "nginx-a2b3c4" => "nginx"
		}

		// Otherwise, it's a "bare" ReplicaSet or DaemonSet—just return parentName.
		return parentName
	}

	// 3) If none of the patterns matched, return the service name as fallback
	if fallbackServiceName != "" {
		return fallbackServiceName
	}

	// 4) Finally return the full pod name (I don't think this will happen)
	return podName
}

const IPPortPattern = `^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d+)$`

var ipPortRegex = regexp.MustCompile(IPPortPattern)

func ExtractIPPort(ipPort string) (string, string, bool) {
	match := ipPortRegex.MatchString(ipPort)

	if !match {
		return "", "", false
	}

	result := ipPortRegex.FindStringSubmatch(ipPort)
	if len(result) != 3 {
		return "", "", false
	}

	ip := result[1]
	port := result[2]

	return ip, port, true
}

func GetHostNetworkPorts(pod *corev1.Pod) []string {
	var ports []string
	if !pod.Spec.HostNetwork {
		return ports
	}
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.HostPort != 0 {
				ports = append(ports, strconv.Itoa(int(port.HostPort)))
			}
		}
	}
	return ports
}

func IsIP(ipString string) bool {
	ip := net.ParseIP(ipString)
	return ip != nil
}

// a safe channel which can be closed multiple times
type SafeChannel struct {
	sync.Mutex

	Ch     chan struct{}
	Closed bool
}

func (sc *SafeChannel) Close() {
	sc.Lock()
	defer sc.Unlock()

	if !sc.Closed {
		close(sc.Ch)
		sc.Closed = true
	}
}

// Deleter represents a type that can delete a key from a map after a certain delay.
type Deleter interface {
	DeleteWithDelay(m *sync.Map, key interface{})
}

// TimedDeleter deletes a key after a specified delay.
type TimedDeleter struct {
	Delay time.Duration
}

func (td *TimedDeleter) DeleteWithDelay(m *sync.Map, key interface{}) {
	go func() {
		time.Sleep(td.Delay)
		m.Delete(key)
	}()
}

type UUIDValue interface {
	UUID() string
}

// TimedDeleterWithIDCheck deletes a key from a sync.Map after a specified delay,
// but only if the value associated with the key has not been updated (i.e. its UUID is unchanged).
// Please note TimedDeleterWithIDCheck only work with UUIDValue as value type
type TimedDeleterWithIDCheck struct {
	Delay time.Duration
}

// DeleteWithDelay schedules the deletion of key from map m after the delay.
// It only deletes the key if the value's UUID remains the same.
func (td *TimedDeleterWithIDCheck) DeleteWithDelay(m *sync.Map, key interface{}) {
	// Attempt to load the value for the key.
	deleteVal, ok := m.Load(key)
	if !ok {
		return
	}

	// The stored value must be of type UUIDValue.
	initialVal, ok := deleteVal.(UUIDValue)
	if !ok {
		return
	}

	go func() {
		time.Sleep(td.Delay)
		// Check if the key still exists.
		currentValRaw, ok := m.Load(key)
		if !ok {
			return
		}
		currentVal, ok := currentValRaw.(UUIDValue)
		if !ok {
			return
		}

		// Compare the UUIDs of the initial value and the current value.
		if currentVal.UUID() == initialVal.UUID() {
			m.Delete(key)
		}
	}()
}
