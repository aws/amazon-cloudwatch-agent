// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package downloader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws/v2"
	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/constants"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	locationDefault = "default"
	locationSSM     = "ssm"
	locationFile    = "file"

	locationSeparator = ":"
)

func RunDownloaderFromFlags(flags map[string]*string) error {
	return RunDownloader(
		*flags["mode"],
		*flags["download-source"],
		*flags["output-dir"],
		*flags["config"],
		*flags["multi-config"],
		*flags["dualstack"] == "true",
	)
}

func RunDownloader(mode, downloadLocation, outputDir, inputConfig, multiConfig string, useDualStack bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fail to fetch the config")
		}
	}()

	cc := commonconfig.New()
	if inputConfig != "" {
		f, err := os.Open(inputConfig)
		if err != nil {
			return fmt.Errorf("failed to open Common Config: %v", err)
		}
		defer f.Close()

		if err := cc.Parse(f); err != nil {
			return fmt.Errorf("failed to parse Common Config: %v", err)
		}
	}

	// Set proxy and SSL environment
	util.SetProxyEnv(cc.ProxyMap())
	util.SetSSLEnv(cc.SSLMap())

	if useDualStack {
		os.Setenv(envconfig.AWS_USE_DUALSTACK_ENDPOINT, "true")
	}

	// Validate required parameters
	if downloadLocation == "" || outputDir == "" {
		executable, err := os.Executable()
		if err == nil {
			return fmt.Errorf("usage: %s --output-dir <path> --download-source ssm:<parameter-store-name>",
				filepath.Base(executable))
		}
		return fmt.Errorf("usage: --output-dir <path> --download-source ssm:<parameter-store-name>")
	}

	// Detect agent mode and region
	mode = util.DetectAgentMode(mode)
	region, _ := util.DetectRegion(mode, cc.CredentialsMap())
	if region == "" && downloadLocation != locationDefault {
		if mode == translatorconfig.ModeEC2 {
			return fmt.Errorf("please check if you can access the metadata service. For example, on linux, run 'wget -q -O - http://169.254.169.254/latest/meta-data/instance-id && echo'")
		}
		return fmt.Errorf("please make sure the credentials and region set correctly on your hosts")
	}

	err = cleanupOutputDir(outputDir)
	if err != nil {
		return fmt.Errorf("failed to clean up output directory: %v", err)
	}

	locationArray := strings.SplitN(downloadLocation, locationSeparator, 2)
	if locationArray == nil || len(locationArray) < 2 && downloadLocation != locationDefault {
		return fmt.Errorf("downloadLocation %s is malformed", downloadLocation)
	}

	var config, outputFilePath string
	switch locationArray[0] {
	case locationDefault:
		outputFilePath = locationDefault
		if multiConfig != "remove" {
			config, err = defaultJSONConfig(mode)
		}
	case locationSSM:
		outputFilePath = locationSSM + "_" + EscapeFilePath(locationArray[1])
		if multiConfig != "remove" {
			config, err = downloadFromSSM(region, locationArray[1], mode, cc.CredentialsMap())
		}
	case locationFile:
		outputFilePath = locationFile + "_" + EscapeFilePath(filepath.Base(locationArray[1]))
		if multiConfig != "remove" {
			config, err = readFromFile(locationArray[1])
		}
	default:
		return fmt.Errorf("location type %s is not supported", locationArray[0])
	}

	if err != nil {
		return fmt.Errorf("fail to fetch/remove json config: %v", err)
	}

	if multiConfig != "remove" {
		outputPath := filepath.Join(outputDir, outputFilePath+constants.FileSuffixTmp)
		// #nosec G306 - customers may need to be able to read the config file that the downloader downloaded for them
		if err := os.WriteFile(outputPath, []byte(config), 0644); err != nil {
			return fmt.Errorf("failed to write the json file %v: %v", outputPath, err)
		}
	} else {
		outputPath := filepath.Join(outputDir, outputFilePath)
		if err := os.Remove(outputPath); err != nil {
			return fmt.Errorf("failed to remove the json file %v: %v", outputPath, err)
		}
	}

	return nil
}

func defaultJSONConfig(mode string) (string, error) {
	return translatorconfig.DefaultJsonConfig(translatorconfig.ToValidOs(""), mode), nil
}

func downloadFromSSM(region, parameterStoreName, mode string, credsConfig map[string]string) (string, error) {
	ctx := context.Background()

	// Build v2 credentials config
	credsMap := util.GetCredentials(mode, credsConfig)
	credConfig := &configaws.CredentialsConfig{
		Region:   region,
		Profile:  credsMap[commonconfig.CredentialProfile],
		Filename: credsMap[commonconfig.CredentialFile],
	}

	cfg, err := credConfig.LoadConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("error in creating session: %v", err)
	}

	ssmClient := ssm.NewFromConfig(cfg)
	input := &ssm.GetParameterInput{
		Name:           aws.String(parameterStoreName),
		WithDecryption: aws.Bool(true),
	}
	output, err := ssmClient.GetParameter(ctx, input)
	if err != nil {
		return "", fmt.Errorf("error in retrieving parameter store content: %v", err)
	}

	return *output.Parameter.Value, nil
}

func readFromFile(filePath string) (string, error) {
	bytes, err := os.ReadFile(filePath)
	return string(bytes), err
}

func EscapeFilePath(filePath string) string {
	escapedFilePath := filepath.ToSlash(filePath)
	escapedFilePath = strings.ReplaceAll(escapedFilePath, "/", "_")
	escapedFilePath = strings.ReplaceAll(escapedFilePath, " ", "_")
	escapedFilePath = strings.ReplaceAll(escapedFilePath, ":", "_")
	return escapedFilePath
}

func cleanupOutputDir(outputDir string) error {
	return filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("cannot access %v: %v", path, err)
		}
		if info.IsDir() {
			if strings.EqualFold(path, outputDir) {
				return nil
			}
			return filepath.SkipDir
		}
		if filepath.Ext(path) == constants.FileSuffixTmp {
			return os.Remove(path)
		}
		return nil
	})
}
