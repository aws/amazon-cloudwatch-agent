// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	commonconfig "github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
	sdkutil "github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	locationDefault = "default"
	locationSSM     = "ssm"
	locationFile    = "file"

	locationSeparator = ":"

	exitErrorMessage = "Fail to fetch the config!"
)

func defaultJsonConfig(mode string) (string, error) {
	return config.DefaultJsonConfig(config.ToValidOs(""), mode), nil
}

func downloadFromSSM(region, parameterStoreName, mode string, credsConfig map[string]string) (string, error) {
	fmt.Printf("Region: %v\n", region)
	fmt.Printf("credsConfig: %v\n", credsConfig)
	var ses *session.Session
	credsMap := util.GetCredentials(mode, credsConfig)
	profile, profileOk := credsMap[commonconfig.CredentialProfile]
	sharedConfigFile, sharedConfigFileOk := credsMap[commonconfig.CredentialFile]
	rootconfig := &aws.Config{
		Region:   aws.String(region),
		LogLevel: configaws.SDKLogLevel(),
		Logger:   configaws.SDKLogger{},
	}
	if profileOk || sharedConfigFileOk {
		rootconfig.Credentials = credentials.NewCredentials(&credentials.SharedCredentialsProvider{
			Filename: sharedConfigFile,
			Profile:  profile,
		})
	}

	ses, err := session.NewSession(rootconfig)
	if err != nil {
		fmt.Printf("Error in creating session: %v\n", err)
		return "", err
	}

	ssmClient := ssm.New(ses)
	input := ssm.GetParameterInput{
		Name:           aws.String(parameterStoreName),
		WithDecryption: aws.Bool(true),
	}
	output, err := ssmClient.GetParameter(&input)
	if err != nil {
		fmt.Printf("Error in retrieving parameter store content: %v\n", err)
		return "", err
	}

	return *output.Parameter.Value, nil
}

func readFromFile(filePath string) (string, error) {
	bytes, err := os.ReadFile(filePath)
	return string(bytes), err
}

func EscapeFilePath(filePath string) (escapedFilePath string) {
	escapedFilePath = filepath.ToSlash(filePath)
	escapedFilePath = strings.Replace(escapedFilePath, "/", "_", -1)
	escapedFilePath = strings.Replace(escapedFilePath, " ", "_", -1)
	escapedFilePath = strings.Replace(escapedFilePath, ":", "_", -1)
	return
}

/**
 *		multi-config:
 *			default, append: download config to the dir and append .tmp suffix
 *			remove: remove the config from the dir
 */
func main() {

	defer func() {
		if r := recover(); r != nil {
			if val, ok := r.(string); ok {
				fmt.Println(val)
			}
			fmt.Println(exitErrorMessage)
			os.Exit(1)
		}
	}()

	var region, mode, downloadLocation, outputDir, inputConfig, multiConfig string

	flag.StringVar(&mode, "mode", "ec2", "Please provide the mode, i.e. ec2, onPremise, onPrem, auto")
	flag.StringVar(&downloadLocation, "download-source", "",
		"Download source. Example: \"ssm:my-parameter-store-name\" for an EC2 SSM Parameter Store Name holding your CloudWatch Agent configuration.")
	flag.StringVar(&outputDir, "output-dir", "", "Path of output json config directory.")
	flag.StringVar(&inputConfig, "config", "", "Please provide the common-config file")
	flag.StringVar(&multiConfig, "multi-config", "default", "valid values: default, append, remove")
	flag.Parse()

	cc := commonconfig.New()
	if inputConfig != "" {
		f, err := os.Open(inputConfig)
		if err != nil {
			log.Panicf("E! Failed to open Common Config: %v", err)
		}

		if err := cc.Parse(f); err != nil {
			log.Panicf("E! Failed to open Common Config: %v", err)
		}
	}
	util.SetProxyEnv(cc.ProxyMap())
	util.SetSSLEnv(cc.SSLMap())
	var errorMessage string
	if downloadLocation == "" || outputDir == "" {
		executable, err := os.Executable()
		if err == nil {
			errorMessage = fmt.Sprintf("E! usage: " + filepath.Base(executable) + " --output-dir <path> --download-source ssm:<parameter-store-name> ")
		} else {
			errorMessage = fmt.Sprintf("E! usage: --output-dir <path> --download-source ssm:<parameter-store-name> ")
		}
		log.Panicf(errorMessage)
	}

	mode = sdkutil.DetectAgentMode(mode)

	region, _ = util.DetectRegion(mode, cc.CredentialsMap())

	if region == "" && downloadLocation != locationDefault {
		fmt.Println("Unable to determine aws-region.")
		if mode == config.ModeEC2 {
			errorMessage = "E! Please check if you can access the metadata service. For example, on linux, run 'wget -q -O - http://169.254.169.254/latest/meta-data/instance-id && echo' "
		} else {
			errorMessage = "E! Please make sure the credentials and region set correctly on your hosts.\n" +
				"Refer to http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html"
		}
		log.Panicf(errorMessage)
	}

	// clean up output dir for tmp files before writing out new tmp file.
	// this step cannot be in translator because it is too late at that time.
	filepath.Walk(
		outputDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Printf("Cannot access %v: %v \n", path, err)
				return err
			}
			if info.IsDir() {
				if strings.EqualFold(path, outputDir) {
					return nil
				} else {
					fmt.Printf("Sub dir %v will be ignored.", path)
					return filepath.SkipDir
				}
			}
			if filepath.Ext(path) == context.TmpFileSuffix {
				return os.Remove(path)
			}
			return nil
		})

	locationArray := strings.SplitN(downloadLocation, locationSeparator, 2)
	if locationArray == nil || len(locationArray) < 2 && downloadLocation != locationDefault {
		log.Panicf("E! downloadLocation %s is malformated.", downloadLocation)
	}

	var config, outputFilePath string
	var err error
	switch locationArray[0] {
	case locationDefault:
		outputFilePath = locationDefault
		if multiConfig != "remove" {
			config, err = defaultJsonConfig(mode)
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
		log.Panicf("E! Location type %s is not supported.", locationArray[0])
	}

	if err != nil {
		log.Panicf("E! Fail to fetch/remove json config: %v", err)
	}

	if multiConfig != "remove" {
		outputFilePath = filepath.Join(outputDir, outputFilePath+context.TmpFileSuffix)
		err = os.WriteFile(outputFilePath, []byte(config), 0644)
		if err != nil {
			log.Panicf("E! Failed to write the json file %v: %v", outputFilePath, err)
		} else {
			fmt.Printf("Successfully fetched the config and saved in %s\n", outputFilePath)
		}
	} else {
		outputFilePath = filepath.Join(outputDir, outputFilePath)
		if err := os.Remove(outputFilePath); err != nil {
			log.Panicf("E! Failed to remove the json file %v: %v", outputFilePath, err)
		} else {
			fmt.Printf("Successfully removed the config file %s\n", outputFilePath)
		}
	}
}
