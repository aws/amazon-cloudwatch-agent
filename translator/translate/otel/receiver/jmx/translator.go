// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	jarPath             = "jar_path"
	targetSystem        = "target_system"
	username            = "username"
	password            = "password"
	keystorePath        = "keystore_path"
	keystorePassword    = "keystore_password"
	keystoreType        = "keystore_type"
	truststorePath      = "truststore_path"
	truststorePassword  = "truststore_password"
	truststoreType      = "truststore_type"
	remoteProfile       = "remote_profile"
	realm               = "realm"
	resourceAttributes  = "resource_attributes"
	timeout             = "timeout"
	headers             = "headers"
	defaultOTLPEndpoint = "127.0.0.1:3000"
	defaultJMXJarPath   = "/opt/aws/amazon-cloudwatch-agent/bin/opentelemetry-jmx-metrics.jar"

	defaultJMXJarPathWin = "C:\\Program Files\\Amazon\\AmazonCloudWatchAgent\\opentelemetry-jmx-metrics.jar"
	defaultTargetSystem = "activemq,cassandra,hbase,hadoop,jetty,jvm,kafka,kafka-consumer,kafka-producer,solr,tomcat,wildfly"
)

var (
	configKeys = map[component.DataType]string{
		component.DataTypeMetrics: common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey),
	}
	redactedMap = make(map[string]string)
)

type translator struct {
	name     string
	dataType component.DataType
	factory  receiver.Factory
	index    int
}

type Option interface {
	apply(t *translator)
}

type optionFunc func(t *translator)

func (o optionFunc) apply(t *translator) {
	o(t)
}

// WithDataType determines where the translator should look to find
// the configuration.
func WithDataType(dataType component.DataType) Option {
	return optionFunc(func(t *translator) {
		t.dataType = dataType
	})
}

func WithIndex(index int) Option {
	return optionFunc(func(t *translator) {
		t.index = index
	})
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	return NewTranslatorWithName("", opts...)
}

func NewTranslatorWithName(name string, opts ...Option) common.Translator[component.Config] {
	t := &translator{name: name, index: -1, factory: jmxreceiver.NewFactory()}
	for _, opt := range opts {
		opt.apply(t)
	}
	if name == "" && t.dataType != "" {
		t.name = string(t.dataType)
		if t.index != -1 {
			t.name += "/" + strconv.Itoa(t.index)
		}
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	configKey, ok := configKeys[t.dataType]
	if !ok {
		return nil, fmt.Errorf("no config key defined for data type: %s", t.dataType)
	}
	if conf == nil || !conf.IsSet(configKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*jmxreceiver.Config)

	var jmxKeyMap map[string]interface{}
	if jmxSlice := common.GetArray[any](conf, configKey); t.index != -1 && len(jmxSlice) > t.index {
		jmxKeyMap = jmxSlice[t.index].(map[string]interface{})
	} else if _, ok := conf.Get(configKey).(map[string]interface{}); !ok {
		jmxKeyMap = make(map[string]interface{})
	} else {
		jmxKeyMap = conf.Get(configKey).(map[string]interface{})
	}

	cfg.JARPath = defaultJMXJarPath
	if runtime.GOOS == "windows" {
		cfg.JARPath = defaultJMXJarPathWin
	}
	if jarPath, ok := jmxKeyMap[jarPath].(string); ok {
		cfg.JARPath = jarPath
	}

	if endpoint, ok := jmxKeyMap[common.Endpoint].(string); ok {
		cfg.Endpoint = endpoint
	}

	cfg.TargetSystem = defaultTargetSystem
	if targetSystem, ok := jmxKeyMap[targetSystem].(string); ok {
		cfg.TargetSystem = targetSystem
	}

	// Prioritize metric collection internal in JMX section, then agent section
	// Setting default to 10 seconds which is used by OTEL as well
	intervalKeyChain := []string{
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, 10*time.Second)
	collectionInterval, err := common.ParseDuration(jmxKeyMap[common.MetricsCollectionIntervalKey])
	if err == nil {
		cfg.CollectionInterval = collectionInterval
	}

	if username, ok := jmxKeyMap[username].(string); ok {
		cfg.Username = username
	}

	if pass, ok := jmxKeyMap[password].(string); ok {
		t.addRedactedMap(password, pass)
		cfg.Password = configopaque.String(pass)
	}

	if keystorePath, ok := jmxKeyMap[keystorePath].(string); ok {
		cfg.KeystorePath = keystorePath
	}

	if keystorePass, ok := jmxKeyMap[keystorePassword].(string); ok {
		t.addRedactedMap(keystorePassword, keystorePass)
		cfg.KeystorePassword = configopaque.String(keystorePass)
	}

	if keystoreType, ok := jmxKeyMap[keystoreType].(string); ok {
		cfg.KeystoreType = keystoreType
	}

	if truststorePath, ok := jmxKeyMap[truststorePath].(string); ok {
		cfg.TruststorePath = truststorePath
	}

	if truststorePass, ok := jmxKeyMap[truststorePassword].(string); ok {
		t.addRedactedMap(truststorePassword, truststorePass)
		cfg.TruststorePassword = configopaque.String(truststorePass)
	}

	if truststoreType, ok := jmxKeyMap[truststoreType].(string); ok {
		cfg.TruststoreType = truststoreType
	}

	if remoteProfile, ok := jmxKeyMap[remoteProfile].(string); ok {
		cfg.RemoteProfile = remoteProfile
	}

	if realm, ok := jmxKeyMap[realm].(string); ok {
		cfg.Realm = realm
	}

	if resourceAttributes, ok := jmxKeyMap[resourceAttributes].(map[string]interface{}); ok {
		cfg.ResourceAttributes = convertToStringMap(resourceAttributes)
	}

	// set OTLP settings
	cfg.OTLPExporterConfig.Endpoint = defaultOTLPEndpoint
	if otlpMap, ok := jmxKeyMap[common.OtlpKey].(map[string]interface{}); ok {
		if endpoint, ok := otlpMap[common.Endpoint].(string); ok {
			cfg.OTLPExporterConfig.Endpoint = endpoint
		}
		timeout, err := common.ParseDuration(otlpMap[timeout])
		if err == nil {
			cfg.OTLPExporterConfig.Timeout = timeout
		}
		if headers, ok := otlpMap[headers].(map[string]interface{}); ok {
			cfg.OTLPExporterConfig.Headers = convertToStringMap(headers)
		}
	}

	return cfg, nil
}

func convertToStringMap(input map[string]interface{}) map[string]string {
	convertedMap := make(map[string]string)
	for key, value := range input {
		strKey := fmt.Sprintf("%v", key)
		strValue := fmt.Sprintf("%v", value)
		convertedMap[strKey] = strValue
	}
	return convertedMap
}

func (t *translator) addRedactedMap(postfix string, value string) {
	redactedMap[fmt.Sprintf("%v", t.factory.Type())+"/"+t.name+"/"+postfix] = value
}

func GetRedactedMap(prefix string, key string) string {
	return redactedMap[prefix+"/"+key]
}
