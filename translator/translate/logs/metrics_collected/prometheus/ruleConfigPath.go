// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"log"
	"os"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	source_ENV      = "env:"
	source_FILE     = "file:"
	sourceSeparator = ":"

	yamlFileMode = 0644

	SectionKeyConfigPath = "prometheus_config_path"
	defaultLinuxPath     = "/etc/cwagentconfig/prometheus.yaml"
	linuxDownloadingFile = "/opt/aws/amazon-cloudwatch-agent/etc/prometheus.yaml"

	windowsDownloadingFile = "\\Amazon\\AmazonCloudWatchAgent\\prometheus.yaml"
)

type ConfigPath struct {
}

func splitConfigPath(configPath string) string {
	locationArray := strings.SplitN(configPath, sourceSeparator, 2)
	if locationArray == nil || len(locationArray) < 2 {
		log.Panicf("Prometheus config path: %s is malformated.", configPath)
	}

	return locationArray[1]
}

func getDownloadPath() string {
	if translator.GetTargetPlatform() == config.OS_TYPE_WINDOWS {
		var downloadingPath string
		if _, ok := os.LookupEnv("ProgramData"); ok {
			downloadingPath = os.Getenv("ProgramData")
		}
		return downloadingPath + windowsDownloadingFile
	}
	return linuxDownloadingFile
}

func (obj *ConfigPath) ApplyRule(input interface{}) (string, interface{}) {

	_, returnVal := translator.DefaultCase(SectionKeyConfigPath, defaultLinuxPath, input)
	configPath := returnVal.(string)

	if strings.HasPrefix(configPath, source_FILE) {
		return SectionKeyConfigPath, splitConfigPath(configPath)
	} else if strings.HasPrefix(configPath, source_ENV) {
		// download the source file to local directory from ENV variable
		downloadingPath := getDownloadPath()
		configEnv := splitConfigPath(configPath)
		if cc, ok := os.LookupEnv(configEnv); ok {
			if error := os.WriteFile(downloadingPath, []byte(cc), yamlFileMode); error != nil {
				log.Panicf("Failed to download the Prometheus config yaml file. Reason: %s", error.Error())
			} else {
				log.Printf("Downloaded the prometheus config from ENV: %v.", configEnv)
			}
		} else {
			log.Panicf("Failed to download the Prometheus config yaml from ENV: %v. Reason: ENV does not exist", configEnv)
		}
		return SectionKeyConfigPath, downloadingPath
	}
	return SectionKeyConfigPath, returnVal
}

func init() {
	obj := new(ConfigPath)
	RegisterRule(SectionKeyConfigPath, obj)
}
