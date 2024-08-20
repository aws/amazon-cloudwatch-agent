// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resolver

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

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
	// if a pod is launched directly by a replicaSet (with a given name by users), its name has the following pattern:
	// Pod name = ReplicaSet name + 5 alphanumeric characters long string
	podWithReplicaSetNamePattern = fmt.Sprintf(`^(.+)-[%s]{5}$`, kubeAllowedStringAlphaNums)
	replicaSetFromPodPattern     = regexp.MustCompile(podWithReplicaSetNamePattern)
)

func attachNamespace(resourceName, namespace string) string {
	// character "@" is not allowed in kubernetes resource names: https://unofficial-kubernetes.readthedocs.io/en/latest/concepts/overview/working-with-objects/names/
	return resourceName + "@" + namespace
}

func getServiceAndNamespace(service *corev1.Service) string {
	return attachNamespace(service.Name, service.Namespace)
}

func extractResourceAndNamespace(serviceOrWorkloadAndNamespace string) (string, string) {
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
	match := replicaSetFromPodPattern.FindStringSubmatch(podName)
	if match != nil {
		return match[1], nil
	}

	return "", errors.New("failed to extract workload name from pod name: " + podName)
}

func getWorkloadAndNamespace(pod *corev1.Pod) string {
	var workloadAndNamespace string
	if pod.ObjectMeta.OwnerReferences != nil {
		for _, ownerRef := range pod.ObjectMeta.OwnerReferences {
			if workloadAndNamespace != "" {
				break
			}

			if ownerRef.Kind == "ReplicaSet" {
				if workloadName, err := extractWorkloadNameFromRS(ownerRef.Name); err == nil {
					// when the replicaSet is created by a deployment, use deployment name
					workloadAndNamespace = attachNamespace(workloadName, pod.Namespace)
				} else if workloadName, err := extractWorkloadNameFromPodName(pod.Name); err == nil {
					// when the replicaSet is not created by a deployment, use replicaSet name directly
					workloadAndNamespace = attachNamespace(workloadName, pod.Namespace)
				}
			} else if ownerRef.Kind == "StatefulSet" {
				workloadAndNamespace = attachNamespace(ownerRef.Name, pod.Namespace)
			} else if ownerRef.Kind == "DaemonSet" {
				workloadAndNamespace = attachNamespace(ownerRef.Name, pod.Namespace)
			}
		}
	}

	return workloadAndNamespace
}

const IP_PORT_PATTERN = `^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d+)$`

var ipPortRegex = regexp.MustCompile(IP_PORT_PATTERN)

func extractIPPort(ipPort string) (string, string, bool) {
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

func getHostNetworkPorts(pod *corev1.Pod) []string {
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

func isIP(ipString string) bool {
	ip := net.ParseIP(ipString)
	return ip != nil
}
