// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"golang.org/x/exp/slices"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/ec2taggerprocessor"
)

const (
	usernameKey               = "username"
	keystorePathKey           = "keystore_path"
	keystoreTypeKey           = "keystore_type"
	truststorePathKey         = "truststore_path"
	truststoreTypeKey         = "truststore_type"
	registrySSLEnabledKey     = "registry_ssl_enabled"
	remoteProfileKey          = "remote_profile"
	realmKey                  = "realm"
	passwordFileKey           = "password_file"
	defaultCollectionInterval = 10 * time.Second
	envJmxJarPath             = "JMX_JAR_PATH"
	attributeHost             = "host"
)

var (
	errNoEndpoint      = errors.New("no endpoint configured")
	errNoTargetSystems = errors.New("no target systems configured")

	localhost = collections.NewSet("localhost", "127.0.0.1")
)

type translator struct {
	name    string
	factory receiver.Factory
	index   int
}

type Option func(any)

func WithIndex(index int) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.index = index
		}
	}
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	t := &translator{index: -1, factory: jmxreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	if t.index != -1 {
		t.name = strconv.Itoa(t.index)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.JmxConfigKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.JmxConfigKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*jmxreceiver.Config)

	jmxMap := common.GetIndexedMap(conf, common.JmxConfigKey, t.index)

	cfg.JARPath = paths.JMXJarPath
	if jarPath := os.Getenv(envJmxJarPath); jarPath != "" {
		cfg.JARPath = jarPath
	}

	if endpoint, ok := jmxMap[common.Endpoint].(string); ok {
		cfg.Endpoint = endpoint
	} else {
		return nil, errNoEndpoint
	}

	var targetSystems []string
	for _, jmxTarget := range common.JmxTargets {
		if _, ok := jmxMap[jmxTarget]; ok {
			targetSystems = append(targetSystems, jmxTarget)
		}
	}
	if len(targetSystems) == 0 {
		return nil, errNoTargetSystems
	}
	cfg.TargetSystem = strings.Join(targetSystems, ",")

	// Prioritize metric collection internal in JMX section, then agent section
	// Setting default to 10 seconds which is used by OTEL as well
	intervalKeyChain := []string{
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, defaultCollectionInterval)
	collectionInterval, err := common.ParseDuration(jmxMap[common.MetricsCollectionIntervalKey])
	if err == nil {
		cfg.CollectionInterval = collectionInterval
	}

	if username, ok := jmxMap[usernameKey].(string); ok {
		cfg.Username = username
	}

	if passwordFile, ok := jmxMap[passwordFileKey].(string); ok {
		cfg.PasswordFile = passwordFile
	}

	if keystorePath, ok := jmxMap[keystorePathKey].(string); ok {
		cfg.KeystorePath = keystorePath
	}

	if keystoreType, ok := jmxMap[keystoreTypeKey].(string); ok {
		cfg.KeystoreType = keystoreType
	}

	if truststorePath, ok := jmxMap[truststorePathKey].(string); ok {
		cfg.TruststorePath = truststorePath
	}

	if registrySSLEnabled, ok := jmxMap[registrySSLEnabledKey].(bool); ok {
		cfg.JMXRegistrySSLEnabled = registrySSLEnabled
	}

	if truststoreType, ok := jmxMap[truststoreTypeKey].(string); ok {
		cfg.TruststoreType = truststoreType
	}

	if remoteProfile, ok := jmxMap[remoteProfileKey].(string); ok {
		cfg.RemoteProfile = remoteProfile
	}

	if realm, ok := jmxMap[realmKey].(string); ok {
		cfg.Realm = realm
	}

	cfg.ResourceAttributes = make(map[string]string)
	if appendDimensions, ok := jmxMap[common.AppendDimensionsKey].(map[string]any); ok {
		c := confmap.NewFromStringMap(appendDimensions)
		if err = c.Unmarshal(&cfg.ResourceAttributes); err != nil {
			return nil, fmt.Errorf("unable to unmarshal %s: %w", common.ConfigKey(common.JmxConfigKey, common.AppendDimensionsKey), err)
		}
	}

	if !context.CurrentContext().GetOmitHostname() && !conf.IsSet(ec2taggerprocessor.Ec2taggerKey) {
		hostname, err := os.Hostname()
		if err != nil {
			log.Printf("E! error finding hostname for jmx metrics %v", err)
		} else {
			cfg.ResourceAttributes[attributeHost] = hostname
		}
	}

	var skipAuthValidation bool
	if insecure, ok := jmxMap[common.InsecureKey].(bool); ok {
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
	for key, value := range map[string]string{
		usernameKey:       cfg.Username,
		passwordFileKey:   cfg.PasswordFile,
		keystorePathKey:   cfg.KeystorePath,
		keystoreTypeKey:   cfg.KeystoreType,
		truststorePathKey: cfg.TruststorePath,
		truststoreTypeKey: cfg.TruststoreType,
	} {
		if value == "" {
			missingFields = append(missingFields, key)
		}
	}
	if missingFields != nil {
		slices.Sort(missingFields)
		return &missingFieldsError{missingFields}
	}
	return nil
}
