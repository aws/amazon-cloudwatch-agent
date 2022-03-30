// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfprocessor

import (
	"fmt"
	"io/ioutil"
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
		errorMessage := fmt.Sprintf("Prometheus config path: %s is malformated.\n", configPath)
		log.Printf(errorMessage)
		panic(errorMessage)
	}

	return locationArray[1]
}

func getDownloadPath() string {
	if translator.GetTargetPlatform() == config.OS_TYPE_WINDOWS {
		var downloadingPath string
		if _, ok := os.LookupEnv("ProgramData"); ok {
			downloadingPath = os.Getenv("ProgramData")
		} else {
			// Windows 2003
			downloadingPath = os.Getenv("ALLUSERSPROFILE") + "\\Application Data"
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
			if error := ioutil.WriteFile(downloadingPath, []byte(cc), yamlFileMode); error != nil {
				errorMessage := fmt.Sprintf("Failed to download the Prometheus config yaml file. Reason: %s \n", error.Error())
				log.Printf(errorMessage)
				panic(errorMessage)
			} else {
				log.Printf("Downloaded the prometheus config from ENV: %v.", configEnv)
			}
		} else {
			errorMessage := fmt.Sprintf("Failed to download the Prometheus config yaml from ENV: %v. Reason: ENV does not exist \n", configEnv)
			log.Printf(errorMessage)
			panic(errorMessage)
		}
		return SectionKeyConfigPath, downloadingPath
	}
	return SectionKeyConfigPath, returnVal
}

func init() {
	obj := new(ConfigPath)
	RegisterRule(SectionKeyConfigPath, obj)
}
