// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/mapstructure"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toenvconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/totomlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toyamlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel"
	translatorUtil "github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	fileMode                 = 0644
	jsonTemplateName_Linux   = "default_linux_config.json"
	jsonTemplateName_Windows = "default_windows_config.json"
	jsonTemplateName_Darwin  = "default_darwin_config.json"
	defaultTomlConfigName    = "CWAgent.conf"
)

// TranslateJsonMapToEnvConfigFile populates env-config.json based on the input json config.
func TranslateJsonMapToEnvConfigFile(jsonConfigValue map[string]interface{}, envConfigPath string) {
	if envConfigPath == "" {
		return
	}
	bytes := toenvconfig.ToEnvConfig(jsonConfigValue)
	if err := os.WriteFile(envConfigPath, bytes, 0644); err != nil {
		log.Panicf("E! Failed to create env config. Reason: %s", err.Error())
	}
}

func getCurBinaryPath() string {
	ex, err := os.Executable()
	if err != nil {
		log.Panicf("E! Failed to get executable path because of %v", err)
	}
	return filepath.Dir(ex)
}

func getJsonConfigMap(jsonConfigFilePath, osType string) (map[string]interface{}, error) {
	if jsonConfigFilePath == "" {
		curPath := getCurBinaryPath()
		if osType == config.OS_TYPE_WINDOWS {
			jsonConfigFilePath = filepath.Join(curPath, jsonTemplateName_Windows)
		} else if osType == config.OS_TYPE_DARWIN {
			jsonConfigFilePath = filepath.Join(curPath, jsonTemplateName_Darwin)
		} else {
			jsonConfigFilePath = filepath.Join(curPath, jsonTemplateName_Linux)
		}
	}
	log.Printf("Reading json config file path: %v ...", jsonConfigFilePath)
	if _, err := os.Stat(jsonConfigFilePath); err != nil {
		fmt.Printf("%v does not exist or cannot read. Skipping it.\n", jsonConfigFilePath)
		return nil, nil
	}

	return translatorUtil.GetJsonMapFromFile(jsonConfigFilePath)
}

func GetTomlConfigPath(tomlFilePath string) string {
	if tomlFilePath == "" {
		curPath := getCurBinaryPath()
		return filepath.Join(curPath, defaultTomlConfigName)
	}
	return tomlFilePath
}

func RunSchemaValidation(inputJsonMap map[string]interface{}) (*gojsonschema.Result, error) {
	schemaLoader := gojsonschema.NewStringLoader(config.GetJsonSchema())
	jsonInputLoader := gojsonschema.NewGoLoader(inputJsonMap)
	return gojsonschema.Validate(schemaLoader, jsonInputLoader)
}

func checkSchema(inputJsonMap map[string]interface{}) {
	result, err := RunSchemaValidation(inputJsonMap)
	if err != nil {
		log.Panicf("E! Failed to run schema validation because of %v", err)
	}
	if result.Valid() {
		log.Print("I! Valid Json input schema.")
	} else {
		errorDetails := result.Errors()
		for _, errorDetail := range errorDetails {
			translator.AddErrorMessages(config.GetFormattedPath(errorDetail.Context().String()), errorDetail.Description())
		}
		log.Panic("E! Invalid Json input schema.")
	}
}

func GenerateMergedJsonConfigMap(ctx *context.Context) (map[string]interface{}, error) {
	// we use a map instead of an array here because we need to override the config value
	// for the append operation when the existing file name and new .tmp file name have diff
	// only for the ".tmp" suffix, i.e. it is override operation even it says append.
	var jsonConfigMapMap = make(map[string]map[string]interface{})

	if ctx.MultiConfig() == "append" || ctx.MultiConfig() == "remove" {
		// backwards compatible for the old json config file
		// this backwards compatible file can be treated as existing files
		jsonConfigMap, err := getJsonConfigMap(ctx.InputJsonFilePath(), ctx.Os())
		if err != nil {
			return nil, fmt.Errorf("unable to get old json config file with error: %v", err)
		}
		if jsonConfigMap != nil {
			jsonConfigMapMap[ctx.InputJsonFilePath()] = jsonConfigMap
		}
	}

	err := filepath.Walk(
		ctx.InputJsonDirPath(),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Printf("Cannot access %v: %v \n", path, err)
				return err
			}
			if envconfig.IsWindowsHostProcessContainer() {
				log.Printf("Skipping checking symbolic link for Windows host process containers %s. \n"+
					"These symbolic links are skipped as valuating symlinks is common failures for Windows containers", path)
			} else {
				if info.Mode()&os.ModeSymlink != 0 {
					log.Printf("Find symbolic link %s \n", path)
					path, err := filepath.EvalSymlinks(path)
					if err != nil {
						log.Printf("Symbolic link %v will be ignored due to err: %v. \n", path, err)
						return nil
					}
					info, err = os.Stat(path)
					if err != nil {
						log.Printf("Path %v will be ignored due to err: %v. \n", path, err)
					}
				}
				if info.IsDir() {
					return nil
				}
			}

			if filepath.Ext(path) == context.TmpFileSuffix {
				// .tmp files
				if ctx.MultiConfig() == "default" || ctx.MultiConfig() == "append" {
					jsonConfigMap, err := getJsonConfigMap(path, ctx.Os())
					if err != nil {
						return err
					}
					if jsonConfigMap != nil {
						jsonConfigMapMap[strings.TrimSuffix(path, context.TmpFileSuffix)] = jsonConfigMap
					}
				}
			} else {
				// non .tmp / existing files
				if ctx.MultiConfig() == "append" || ctx.MultiConfig() == "remove" {
					jsonConfigMap, err := getJsonConfigMap(path, ctx.Os())
					if err != nil {
						return err
					}
					if jsonConfigMap != nil {
						if _, ok := jsonConfigMapMap[path]; !ok {
							jsonConfigMapMap[path] = jsonConfigMap
						}
					}
				}
			}

			return nil
		})
	if err != nil {
		log.Printf("unable to scan config dir %v with error: %v", ctx.InputJsonDirPath(), err)
	}

	if len(jsonConfigMapMap) == 0 {
		// For containerized agent, try to read env variable only when json configuration file is absent
		if jsonConfigContent, ok := os.LookupEnv(config.CWConfigContent); ok && os.Getenv(config.RUN_IN_CONTAINER) == config.RUN_IN_CONTAINER_TRUE {
			log.Printf("Reading json config from from environment variable %v.", config.CWConfigContent)
			jm, err := translatorUtil.GetJsonMapFromJsonBytes([]byte(jsonConfigContent))
			if err != nil {
				return nil, fmt.Errorf("unable to get json map from environment variable %v with error: %v", config.CWConfigContent, err)
			}
			jsonConfigMapMap[config.CWConfigContent] = jm
		}
	}

	defaultConfig, err := translatorUtil.GetDefaultJsonConfigMap(ctx.Os(), ctx.Mode())
	if err != nil {
		return nil, err
	}
	mergedJsonConfigMap, err := jsonconfig.MergeJsonConfigMaps(jsonConfigMapMap, defaultConfig, ctx.MultiConfig())
	if err != nil {
		return nil, err
	}

	// Json Schema Validation by gojsonschema
	checkSchema(mergedJsonConfigMap)
	return mergedJsonConfigMap, nil
}

func TranslateJsonMapToTomlConfig(jsonConfigValue interface{}) (interface{}, error) {
	r := new(translate.Translator)
	_, val := r.ApplyRule(jsonConfigValue)
	if !translator.IsTranslateSuccess() {
		return nil, fmt.Errorf("%v", translator.ErrorMessages)
	}
	// Translation is valid, log info messages and continue to convert/write to toml
	for _, infoMessage := range translator.InfoMessages {
		log.Println(infoMessage)
	}
	return val, nil
}

func TranslateJsonMapToYamlConfig(jsonConfigValue interface{}) (interface{}, error) {
	cfg, err := otel.Translate(jsonConfigValue, context.CurrentContext().Os())
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if result, err = mapstructure.Marshal(cfg); err != nil {
		return nil, err
	}
	return result, nil
}

func ConfigToTomlFile(config interface{}, tomlConfigFilePath string) error {
	res := totomlconfig.ToTomlConfig(config)
	return os.WriteFile(tomlConfigFilePath, []byte(res), fileMode)
}

func ConfigToYamlFile(config interface{}, yamlConfigFilePath string) error {
	res := toyamlconfig.ToYamlConfig(config)
	// If YAML just contains the word null, then remove the file.
	if strings.TrimSpace(res) == "null" {
		// Intentionally not checking return value.
		_ = os.Remove(yamlConfigFilePath)
		return nil
	}
	return os.WriteFile(yamlConfigFilePath, []byte(res), fileMode)
}
