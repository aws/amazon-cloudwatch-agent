package cmdutil

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	userutil "github.com/aws/amazon-cloudwatch-agent/internal/util/user"
	"github.com/aws/amazon-cloudwatch-agent/translator"

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

type ConfigTranslator struct {
	ctx *context.Context
}

func NewConfigTranslator(inputOs, inputJsonFile, inputJsonDir, inputTomlFile, inputMode, inputConfig, multiConfig string) (*ConfigTranslator, error) {

	ct := ConfigTranslator{
		ctx: context.CurrentContext(),
	}

	ct.ctx.SetOs(inputOs)
	ct.ctx.SetInputJsonFilePath(inputJsonFile)
	ct.ctx.SetInputJsonDirPath(inputJsonDir)
	ct.ctx.SetMultiConfig(multiConfig)
	ct.ctx.SetOutputTomlFilePath(inputTomlFile)

	if inputConfig != "" {
		f, err := os.Open(inputConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to open common-config file %s with error: %v", inputConfig, err)
		}
		defer f.Close()
		conf, err := commonconfig.Parse(f)
		if err != nil {
			return nil, fmt.Errorf("failed to parse common-config file %s with error: %v", inputConfig, err)
		}
		ct.ctx.SetCredentials(conf.CredentialsMap())
		ct.ctx.SetProxy(conf.ProxyMap())
		ct.ctx.SetSSL(conf.SSLMap())
		translatorUtil.LoadImdsRetries(conf.IMDS)
	}
	translatorUtil.SetProxyEnv(ct.ctx.Proxy())
	translatorUtil.SetSSLEnv(ct.ctx.SSL())

	mode := translatorUtil.DetectAgentMode(inputMode)
	ct.ctx.SetMode(mode)
	ct.ctx.SetKubernetesMode(translatorUtil.DetectKubernetesMode(mode))

	return &ct, nil
}

func (ct *ConfigTranslator) Translate() error {
	defer func() {
		if r := recover(); r != nil {
			if val, ok := r.(string); ok {
				log.Println(val)
			}
			for _, errMessage := range translator.ErrorMessages {
				log.Println(errMessage)
			}
			log.Printf(exitErrorMessage, version)
		}
	}()

	mergedJsonConfigMap, err := GenerateMergedJsonConfigMap(ct.ctx)
	if err != nil {
		return fmt.Errorf("failed to generate merged json config: %v", err)
	}

	if !ct.ctx.RunInContainer() {
		current, err := user.Current()
		if err == nil && current.Name == "****" {
			runAsUser, err := userutil.DetectRunAsUser(mergedJsonConfigMap)
			if err != nil {
				return fmt.Errorf("failed to detectRunAsUser")
			}
			VerifyCredentials(ct.ctx, runAsUser)
		}
	}

	tomlConfigPath := GetTomlConfigPath(ct.ctx.OutputTomlFilePath())
	tomlConfigDir := filepath.Dir(tomlConfigPath)
	yamlConfigPath := filepath.Join(tomlConfigDir, yamlConfigFileName)
	tomlConfig, err := TranslateJsonMapToTomlConfig(mergedJsonConfigMap)
	if err != nil {
		return fmt.Errorf("failed to generate TOML configuration validation content: %v", err)
	}
	yamlConfig, err := TranslateJsonMapToYamlConfig(mergedJsonConfigMap)
	if err != nil && !errors.Is(err, pipeline.ErrNoPipelines) {
		return fmt.Errorf("failed to generate YAML configuration validation content: %v", err)
	}
	if err = ConfigToTomlFile(tomlConfig, tomlConfigPath); err != nil {
		return fmt.Errorf("failed to create the configuration TOML validation file: %v", err)
	}
	if err = ConfigToYamlFile(yamlConfig, yamlConfigPath); err != nil {
		return fmt.Errorf("failed to create the configuration YAML validation file: %v", err)
	}
	log.Println(exitSuccessMessage)

	envConfigPath := filepath.Join(tomlConfigDir, envConfigFileName)
	TranslateJsonMapToEnvConfigFile(mergedJsonConfigMap, envConfigPath)

	return nil
}
