// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import "os"

const KubernetesEnvVar = "K8S_NAMESPACE"

func IsAppSignalsKubernetes() bool {
	_, isSet := os.LookupEnv(KubernetesEnvVar)
	return isSet
}
