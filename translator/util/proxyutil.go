// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"os"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
)

func GetHttpProxy(proxyConfig map[string]string) (result map[string]string) {
	result = make(map[string]string)
	if val, ok := proxyConfig[commonconfig.HttpProxy]; ok {
		result[commonconfig.HttpProxy] = val
		return
	}
	names := []string{"HTTP_PROXY", "http_proxy"}
	for _, name := range names {
		if val := os.Getenv(name); val != "" {
			result[commonconfig.HttpProxy] = val
			return
		}
	}
	return
}

func GetHttpsProxy(proxyConfig map[string]string) (result map[string]string) {
	result = make(map[string]string)
	if val, ok := proxyConfig[commonconfig.HttpsProxy]; ok {
		result[commonconfig.HttpsProxy] = val
		return
	}
	names := []string{"HTTPS_PROXY", "https_proxy"}
	for _, name := range names {
		if val := os.Getenv(name); val != "" {
			result[commonconfig.HttpsProxy] = val
			return
		}
	}
	return
}

func GetNoProxy(proxyConfig map[string]string) (result map[string]string) {
	result = make(map[string]string)
	if val, ok := proxyConfig[commonconfig.NoProxy]; ok {
		result[commonconfig.NoProxy] = val
		return
	}
	names := []string{"No_PROXY", "no_proxy"}
	for _, name := range names {
		if val := os.Getenv(name); val != "" {
			result[commonconfig.NoProxy] = val
			return
		}
	}
	return
}

func SetProxyEnv(proxyConfig map[string]string) {
	if httpProxy := GetHttpProxy(proxyConfig); len(httpProxy) > 0 {
		os.Setenv("HTTP_PROXY", httpProxy[commonconfig.HttpProxy])
	}
	if httpsProxy := GetHttpsProxy(proxyConfig); len(httpsProxy) > 0 {
		os.Setenv("HTTPS_PROXY", httpsProxy[commonconfig.HttpsProxy])
	}
	if noProxy := GetNoProxy(proxyConfig); len(noProxy) > 0 {
		os.Setenv("NO_PROXY", noProxy[commonconfig.NoProxy])
	}
}
