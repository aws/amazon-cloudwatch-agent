package util

import (
	"errors"
	"fmt"
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

	linuxDownloadingPath   = "/opt/aws/amazon-cloudwatch-agent/etc/"
	windowsDownloadingPath = "\\Amazon\\AmazonCloudWatchAgent\\"
)

func splitConfigPath(configPath string) (string, error) {
	locationArray := strings.SplitN(configPath, sourceSeparator, 2)
	if locationArray == nil || len(locationArray) < 2 {
		return "", errors.New(fmt.Sprintf("config path: %s is malformatted.", configPath))
	}

	return locationArray[1], nil
}

func getDownloadPath() string {
	if translator.GetTargetPlatform() == config.OS_TYPE_WINDOWS {
		var downloadingPath string
		if _, ok := os.LookupEnv("ProgramData"); ok {
			downloadingPath = os.Getenv("ProgramData")
		}
		return downloadingPath + windowsDownloadingPath
	}
	return linuxDownloadingPath
}

func GetConfigPath(configFileName string, sectionKey string, defaultPath string, input interface{}) (interface{}, error) {
	_, returnVal := translator.DefaultCase(sectionKey, defaultPath, input)
	configPath := returnVal.(string)

	if strings.HasPrefix(configPath, source_FILE) {
		return splitConfigPath(configPath)
	} else if strings.HasPrefix(configPath, source_ENV) {
		// download the source file to local directory from ENV variable
		downloadingPath := getDownloadPath() + configFileName
		configEnv, err := splitConfigPath(configPath)
		if err != nil {
			return nil, err
		}
		if cc, ok := os.LookupEnv(configEnv); ok {
			if err := os.WriteFile(downloadingPath, []byte(cc), yamlFileMode); err != nil {
				return "", errors.New(fmt.Sprintf("Failed to download config file %s. Reason: %s", configFileName, err.Error()))
			}
		} else {
			return "", errors.New(fmt.Sprintf("Failed to download config from ENV: %v. Reason: ENV does not exist", configEnv))
		}
		return downloadingPath, nil
	}
	return returnVal, nil
}
