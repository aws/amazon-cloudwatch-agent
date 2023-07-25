// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jsonconfig

import (
	"log"
	"os"
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

func MergeJsonConfigMaps(jsonConfigMapMap map[string]map[string]interface{}, defaultJsonConfigMap map[string]interface{}, multiConfig string) (map[string]interface{}, error) {
	if len(jsonConfigMapMap) == 0 {
		if os.Getenv(config.USE_DEFAULT_CONFIG) == config.USE_DEFAULT_CONFIG_TRUE {
			// When USE_DEFAULT_CONFIG is true, ECS and EKS will be supposed to use different default config. EKS default config logic will be added when necessary
			if ecsutil.GetECSUtilSingleton().IsECS() {
				log.Println("No json config files found, use the default ecs config")
				return util.GetJsonMapFromJsonBytes([]byte(config.DefaultECSJsonConfig()))
			}
		}
		if multiConfig == "remove" {
			os.Exit(config.ERR_CODE_NOJSONFILE)
		} else {
			log.Println("No json config files found, use the default one")
		}
		return defaultJsonConfigMap, nil
	}

	resultMap := map[string]interface{}{}
	/** merge json maps, follow below rules
	 * 1. If it is global config, no conflicts are allowed, i.e. either only one defines the value, or the values defined by multiple parties are the same.
	 * 2. If it is plugin config,
	 *	  a. merge them into one instance if they are exactly the same,
	 *	  b. otherwise, make them as separate instances (as list) if possible,
	 *	  c. fail the operation if list is not allowed for that plugin.
	 */

	keys := make([]string, len(jsonConfigMapMap))
	for key := range jsonConfigMapMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, k := range keys {
		Merge(jsonConfigMapMap[k], resultMap)
	}

	if !translator.IsTranslateSuccess() {
		log.Panic("Failed to merge multiple json config files.")
	}

	return resultMap, nil
}

func Merge(source map[string]interface{}, result map[string]interface{}) {
	for _, rule := range mergeJsonUtil.MergeRuleMap {
		rule.Merge(source, result)
	}
}
