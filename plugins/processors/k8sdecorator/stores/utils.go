// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sutil"
	corev1 "k8s.io/api/core/v1"
	"regexp"
	"strings"
)

func createPodKeyFromMetaData(pod *corev1.Pod) string {
	namespace := pod.Namespace
	podName := pod.Name
	return k8sutil.CreatePodKey(namespace, podName)
}

func createPodKeyFromMetric(tags map[string]string) string {
	namespace := tags[K8sNamespace]
	podName := tags[K8sPodNameKey]
	return k8sutil.CreatePodKey(namespace, podName)
}

func createContainerKeyFromMetric(tags map[string]string) string {
	namespace := tags[K8sNamespace]
	podName := tags[K8sPodNameKey]
	containerName := tags[ContainerNamekey]
	return k8sutil.CreateContainerKey(namespace, podName, containerName)
}

const (
	// kubeAllowedStringAlphaNums holds the characters allowed in replicaset names from as parent deployment
	// https://github.com/kubernetes/apimachinery/blob/master/pkg/util/rand/rand.go#L83
	kubeAllowedStringAlphaNums = "bcdfghjklmnpqrstvwxz2456789"
	cronJobAllowedString       = "0123456789"
)

// get the deployment name by stripping the last dash following some rules
// return empty if it is not following the rule
func parseDeploymentFromReplicaSet(name string) string {
	lastDash := strings.LastIndexAny(name, "-")
	if lastDash == -1 {
		// No dash
		return ""
	}
	suffix := name[lastDash+1:]
	if len(suffix) < 3 {
		// Invalid suffix if it is less than 3
		return ""
	}

	if !stringInRuneset(suffix, kubeAllowedStringAlphaNums) {
		// Invalid suffix
		return ""
	}

	return name[:lastDash]
}

// Get the cronJob name by stripping the last dash following by the naming convention: JobName-UnixTime
// based on https://github.com/kubernetes/kubernetes/blob/c4d752765b3bbac2237bf87cf0b1c2e307844666/pkg/controller/cronjob/cronjob_controllerv2.go#L594-L596.
// Before v1.21 CronJob in Kubernetes has used Unix Time in second; after v1.21 is a Unix Time in Minutes.

func parseCronJobFromJob(name string) string {
	lastDash := strings.LastIndexAny(name, "-")

	//Return empty since the naming convention is: JobName-UnixTime, if it does not have the "-", meanings the job name is empty
	if lastDash == -1 {
		return ""
	}

	suffix := name[lastDash+1:]

	//Checking if the suffix is a unix time by checking: the length and contains character
	if !isUnixTime(suffix) {
		return ""
	}

	return name[:lastDash]
}

func isUnixTime(name string) bool {
	//Checking for the length: CronJobControllerV2 is Unix Time in Minutes (9 characters) while CronJob is Unix Time (10 characters)
	var validUnixTimeRegExp = regexp.MustCompile(`/^\d{9,10}$/`)

	return validUnixTimeRegExp.MatchString(name)

}
