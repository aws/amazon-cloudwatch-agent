// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"os"

	"go.opentelemetry.io/collector/confmap"
)

const KubernetesEnvVar = "K8S_NAMESPACE"

func IsAppSignalsKubernetes() bool {
	_, isSet := os.LookupEnv(KubernetesEnvVar)
	return isSet
}

func GetHostedIn(conf *confmap.Conf) (string, bool) {
	hostedIn, hostedInConfigured := GetString(conf, ConfigKey(LogsKey, MetricsCollectedKey, AppSignals, "hosted_in"))
	if !hostedInConfigured {
		hostedIn, hostedInConfigured = GetString(conf, ConfigKey(LogsKey, MetricsCollectedKey, AppSignalsFallback, "hosted_in"))
	}
	return hostedIn, hostedInConfigured
}
