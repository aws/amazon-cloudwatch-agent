// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	jarPathKey        = "jar_path"
	targetSystemKey   = "target_system"
	usernameKey       = "username"
	keystorePathKey   = "keystore_path"
	keystoreTypeKey   = "keystore_type"
	truststorePathKey = "truststore_path"
	truststoreTypeKey = "truststore_type"
	remoteProfileKey  = "remote_profile"
	realmKey          = "realm"
	passwordFileKey   = "password_file"
	otlpTimeoutKey    = "timeout"
	otlpHeadersKey    = "headers"

	defaultTargetSystem = "activemq,cassandra,hbase,hadoop,jetty,jvm,kafka,kafka-consumer,kafka-producer,solr,tomcat,wildfly"

	envJmxJarPath = "JMX_JAR_PATH"
)

var (
	configKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey)
	localhost = collections.NewSet("localhost", "127.0.0.1")
)

type translator struct {
	name    string
	factory receiver.Factory
	index   int
}

type Option interface {
	apply(t *translator)
}

type optionFunc func(t *translator)

func (o optionFunc) apply(t *translator) {
	o(t)
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
	if name == "" && t.index != -1 {
		t.name = strconv.Itoa(t.index)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(configKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*jmxreceiver.Config)

	var jmxKeyMap map[string]any
	if jmxSlice := common.GetArray[any](conf, configKey); t.index != -1 && len(jmxSlice) > t.index {
		jmxKeyMap = jmxSlice[t.index].(map[string]any)
	} else if m, ok := conf.Get(configKey).(map[string]any); ok {
		jmxKeyMap = m
	}

	cfg.JARPath = paths.JMXJarPath
	if jarPath, ok := jmxKeyMap[jarPathKey].(string); ok {
		cfg.JARPath = jarPath
	} else if os.Getenv(envJmxJarPath) != "" {
		cfg.JARPath = os.Getenv(envJmxJarPath)
	}

	if endpoint, ok := jmxKeyMap[common.Endpoint].(string); ok {
		cfg.Endpoint = endpoint
	}

	cfg.TargetSystem = defaultTargetSystem
	if targetSystem, ok := jmxKeyMap[targetSystemKey].(string); ok {
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

	if username, ok := jmxKeyMap[usernameKey].(string); ok {
		cfg.Username = username
	}

	if passwordFile, ok := jmxKeyMap[passwordFileKey].(string); ok {
		cfg.PasswordFile = passwordFile
	}

	if keystorePath, ok := jmxKeyMap[keystorePathKey].(string); ok {
		cfg.KeystorePath = keystorePath
	}

	if keystoreType, ok := jmxKeyMap[keystoreTypeKey].(string); ok {
		cfg.KeystoreType = keystoreType
	}

	if truststorePath, ok := jmxKeyMap[truststorePathKey].(string); ok {
		cfg.TruststorePath = truststorePath
	}

	if truststoreType, ok := jmxKeyMap[truststoreTypeKey].(string); ok {
		cfg.TruststoreType = truststoreType
	}

	if remoteProfile, ok := jmxKeyMap[remoteProfileKey].(string); ok {
		cfg.RemoteProfile = remoteProfile
	}

	if realm, ok := jmxKeyMap[realmKey].(string); ok {
		cfg.Realm = realm
	}

	if appendDimensions, ok := jmxKeyMap[common.AppendDimensionsKey].(map[string]any); ok {
		c := confmap.NewFromStringMap(appendDimensions)
		if err = c.Unmarshal(&cfg.ResourceAttributes); err != nil {
			return nil, fmt.Errorf("unable to unmarshal %s::%s: %w", configKey, common.AppendDimensionsKey, err)
		}
	}

	// set OTLP settings
	if otlpMap, ok := jmxKeyMap[common.OtlpKey].(map[string]any); ok {
		if endpoint, ok := otlpMap[common.Endpoint].(string); ok {
			cfg.OTLPExporterConfig.Endpoint = endpoint
		}
		timeout, err := common.ParseDuration(otlpMap[otlpTimeoutKey])
		if err == nil {
			cfg.OTLPExporterConfig.Timeout = timeout
		}
		if headers, ok := otlpMap[otlpHeadersKey].(map[string]any); ok {
			c := confmap.NewFromStringMap(headers)
			if err = c.Unmarshal(&cfg.OTLPExporterConfig.Headers); err != nil {
				return nil, fmt.Errorf("unable to unmarshal %s::%s::%s: %w", configKey, common.OtlpKey, otlpHeadersKey, err)
			}
		}
	}

	var skipAuthValidation bool
	if insecure, ok := jmxKeyMap[common.InsecureKey].(bool); ok {
		skipAuthValidation = insecure
	}

	return cfg, validate(cfg, skipAuthValidation)
}

func validate(cfg *jmxreceiver.Config, skipAuthValidation bool) error {
	if !skipAuthValidation && cfg.Endpoint != "" {
		host, _, err := net.SplitHostPort(cfg.Endpoint)
		if err != nil {
			return fmt.Errorf("unable to parse endpoint: %w", err)
		}
		if !localhost.Contains(host) {
			if err = validateAuth(cfg); err != nil {
				return fmt.Errorf("jmx configuration with endpoint (%s): %w", cfg.Endpoint, err)
			}
		}
	}
	return nil
}

type missingFieldsError struct {
	fields []string
}

func (e *missingFieldsError) Error() string {
	return fmt.Sprintf("missing required field(s) for remote access: %v", strings.Join(e.fields, ", "))
}

func validateAuth(cfg *jmxreceiver.Config) error {
	var missingFields []string
	for _, fields := range [][2]string{
		{cfg.Username, usernameKey},
		{cfg.PasswordFile, passwordFileKey},
		{cfg.KeystorePath, keystorePathKey},
		{cfg.KeystoreType, keystoreTypeKey},
		{cfg.TruststorePath, truststorePathKey},
		{cfg.TruststoreType, truststoreTypeKey},
	} {
		field, key := fields[0], fields[1]
		if field == "" {
			missingFields = append(missingFields, key)
		}
	}
	if missingFields != nil {
		return &missingFieldsError{fields: missingFields}
	}
	return nil
}
