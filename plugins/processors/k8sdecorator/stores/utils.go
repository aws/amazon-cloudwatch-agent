// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stores

import (
	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sutil"
	corev1 "k8s.io/api/core/v1"
	"regexp"
	"strconv"
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
	// deploymentUnexpectedRegEx holds the characters allowed in replicaset names from as parent deployment
	// https://github.com/kubernetes/apimachinery/blob/master/pkg/util/rand/rand.go#L83
	deploymentUnexpectedRegEx = `[^b-hj-np-tv-xz-z2-24-9]+`
	// cronJobUnexpectedRegex ensures the characters in cron job name are only numbers.
	cronJobUnexpectedRegex = `^[^0-9]+$`
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

	if containUnexpectedRuneSet(suffix, deploymentUnexpectedRegEx) {
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
	suffixInt, _ := strconv.ParseInt(suffix, 10, 64)
	//Convert Unix Time In Minutes to Unix Time
	suffixStringMultiply := strconv.FormatInt(suffixInt*60, 10)

	//Checking if the suffix is a unix time by checking: the length and contains character
	// - Checking for the length: CronJobControllerV2 is Unix Time in Minutes (7-9 characters) while CronJob is Unix Time (10 characters).
	//   However, multiply by 60 to convert the Unix Time In Minutes back to Unix Time in order to have the same condition as Unix Time
	if len(suffix) != 10 && len(suffixStringMultiply) != 10 {
		return ""
	}

	// - Checking for unexpected character such as having characters others than numbers
	if containUnexpectedRuneSet(suffix, cronJobUnexpectedRegex) || containUnexpectedRuneSet(suffixStringMultiply, cronJobUnexpectedRegex) {
		return ""
	}

	return name[:lastDash]
}

func containUnexpectedRuneSet(name, unexpectedRegEx string) bool {
	var validUnixTimeRegExp = regexp.MustCompile(unexpectedRegEx)
	return validUnixTimeRegExp.MatchString(name)
}
