// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
	translatorUtil "github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	exitErrorMessage   = "Configuration validation first phase failed. Agent version: %v. Verify the JSON input is only using features supported by this version.\n"
	exitSuccessMessage = "Configuration validation first phase succeeded"
	version            = "1.0"
	envConfigFileName  = "env-config.json"
	yamlConfigFileName = "amazon-cloudwatch-agent.yaml"
)

func initFlags() {
	var inputOs = flag.String("os", "", "Please provide the os preference, valid value: windows/linux.")
	var inputJsonFile = flag.String("input", "", "Please provide the path of input agent json config file")
	var inputJsonDir = flag.String("input-dir", "", "Please provide the path of input agent json config directory.")
	var inputTomlFile = flag.String("output", "", "Please provide the path of the output CWAgent config file")
	var inputMode = flag.String("mode", "ec2", "Please provide the mode, i.e. ec2, onPremise, onPrem, auto")
	var inputConfig = flag.String("config", "", "Please provide the common-config file")
	var multiConfig = flag.String("multi-config", "remove", "valid values: default, append, remove")
	flag.Parse()

	ctx := context.CurrentContext()
	ctx.SetOs(*inputOs)
	ctx.SetInputJsonFilePath(*inputJsonFile)
	ctx.SetInputJsonDirPath(*inputJsonDir)
	ctx.SetMultiConfig(*multiConfig)
	ctx.SetOutputTomlFilePath(*inputTomlFile)

	if *inputConfig != "" {
		f, err := os.Open(*inputConfig)
		if err != nil {
			log.Fatalf("E! Failed to open common-config file %s with error: %v", *inputConfig, err)
		}
		defer f.Close()
		conf, err := commonconfig.Parse(f)
		if err != nil {
			log.Fatalf("E! Failed to parse common-config file %s with error: %v", *inputConfig, err)
		}
		ctx.SetCredentials(conf.CredentialsMap())
		ctx.SetProxy(conf.ProxyMap())
		ctx.SetSSL(conf.SSLMap())
		translatorUtil.LoadImdsRetries(conf.IMDS)
	}
	translatorUtil.SetProxyEnv(ctx.Proxy())
	translatorUtil.SetSSLEnv(ctx.SSL())

	mode := translatorUtil.DetectAgentMode(*inputMode)
	ctx.SetMode(mode)
	ctx.SetKubernetesMode(translatorUtil.DetectKubernetesMode(mode))
}

/**
 *	config-translator --input ${JSON} --input-dir ${JSON_DIR} --output ${TOML} --mode ${param_mode} --config ${COMMON_CONFIG}
 *  --multi-config [default|append|remove]
 *
 *		multi-config:
 *			default:	only process .tmp files
 *			append:		process both existing files and .tmp files
 *			remove:		only process existing files
 */
func main() {
	initFlags()
	defer func() {
		if r := recover(); r != nil {
			// Only emit error message if panic content is string(pre-checked)
			// Not emitting the non-handled error message for now, we don't want to show non-user-friendly error message to customer
			if val, ok := r.(string); ok {
				log.Println(val)
			}
			//If the Input JSON config file is invalid, output all the error path and error messages.
			for _, errMessage := range translator.ErrorMessages {
				log.Println(errMessage)
			}
			log.Printf(exitErrorMessage, version)
			os.Exit(1)
		}
	}()
	ctx := context.CurrentContext()

	mergedJsonConfigMap, err := cmdutil.GenerateMergedJsonConfigMap(ctx)
	if err != nil {
		log.Panicf("E! Failed to generate merged json config: %v", err)
	}

	if !ctx.RunInContainer() {
		// run as user only applies to non container situation.
		current, err := user.Current()
		if err == nil && current.Name == "root" {
			runAsUser, err := cmdutil.DetectRunAsUser(mergedJsonConfigMap)
			if err != nil {
				log.Panic("E! Failed to detectRunAsUser")
			}
			cmdutil.VerifyCredentials(ctx, runAsUser)
		}
	}

	tomlConfigPath := cmdutil.GetTomlConfigPath(ctx.OutputTomlFilePath())
	tomlConfigDir := filepath.Dir(tomlConfigPath)
	yamlConfigPath := filepath.Join(tomlConfigDir, yamlConfigFileName)
	tomlConfig, err := cmdutil.TranslateJsonMapToTomlConfig(mergedJsonConfigMap)
	if err != nil {
		log.Panicf("E! Failed to generate TOML configuration validation content: %v", err)
	}
	yamlConfig, err := cmdutil.TranslateJsonMapToYamlConfig(mergedJsonConfigMap)
	if err != nil && !errors.Is(err, pipeline.ErrNoPipelines) {
		log.Panicf("E! Failed to generate YAML configuration validation content: %v", err)
	}
	if err = cmdutil.ConfigToTomlFile(tomlConfig, tomlConfigPath); err != nil {
		log.Panicf("E! Failed to create the configuration TOML validation file: %v", err)
	}
	if err = cmdutil.ConfigToYamlFile(yamlConfig, yamlConfigPath); err != nil {
		log.Panicf("E! Failed to create the configuration YAML validation file: %v", err)
	}
	log.Println(exitSuccessMessage)
	// Put env config into the same folder as the toml config
	envConfigPath := filepath.Join(tomlConfigDir, envConfigFileName)
	cmdutil.TranslateJsonMapToEnvConfigFile(mergedJsonConfigMap, envConfigPath)
}
