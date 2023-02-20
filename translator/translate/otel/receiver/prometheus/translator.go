// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusreceiver

import (
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"gopkg.in/yaml.v3"

	emfprocessor "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected/prometheus"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected/prometheus/ecsservicediscovery"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	factory receiver.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

var prometheusKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)

// NewTranslator creates a new aws container insight receiver translator.
func NewTranslator() common.Translator[component.Config] {
	return &translator{
		factory: prometheusreceiver.NewFactory(),
	}
}

func (t *translator) Type() component.Type {
	return t.factory.Type()
}

// Translate creates a receiver for prometheus if the logs.metrics_collected.prometheus section is present.
func (t *translator) Translate(conf *confmap.Conf, translatorOptions common.TranslatorOptions) (component.Config, error) {
	if conf == nil || !conf.IsSet(prometheusKey) {
		return nil, &common.MissingKeyError{Type: t.Type(), JsonKey: prometheusKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*prometheusreceiver.Config)

	err := t.setPrometheusConfig(conf, cfg)
	if err != nil {
		return nil, err
	}

	err = t.createEmptyEcsSdResultFile(conf)
	if err != nil {
		return nil, err
	}

	err = t.addEcsRelabelConfigs(conf, cfg)
	if err != nil {
		return nil, err
	}

	err = t.copyPrometheusConfigToConfigPlaceholder(cfg) // Should be the last step before returning
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Extract and set the contents of the actual prometheus config yaml
func (t *translator) setPrometheusConfig(conf *confmap.Conf, cfg *prometheusreceiver.Config) error {
	cloneCfg := t.factory.CreateDefaultConfig().(*prometheusreceiver.Config)
	prometheusConfigPathKey := common.ConfigKey(prometheusKey, "prometheus_config_path")
	if conf == nil || !conf.IsSet(prometheusConfigPathKey) {
		return &common.MissingKeyError{Type: t.Type(), JsonKey: prometheusConfigPathKey}
	}
	_, result := new(emfprocessor.ConfigPath).ApplyRule(conf.Get(prometheusKey)) // TODO: remove dependency on rule.
	prometheusConfigPath, ok := result.(string)
	if !ok || result == "" {
		return fmt.Errorf("unable to extract prometheus config path")
	}
	prometheusConfigYaml, err := os.ReadFile(prometheusConfigPath)
	if err != nil || len(prometheusConfigYaml) == 0 {
		return fmt.Errorf("unable to extract prometheus config content from %s", prometheusConfigPath)
	}
	var prometheusConfig map[string]interface{}
	if err := yaml.Unmarshal(prometheusConfigYaml, &prometheusConfig); err != nil {
		return fmt.Errorf("unable to read default config: %w", err)
	}
	c := confmap.NewFromStringMap(map[string]interface{}{
		"config": prometheusConfig, // As per https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/7f4d4425a03e7e47575211be489f912cd16ae509/receiver/prometheusreceiver/README.md?plain=1#L58
	})
	if err := c.Unmarshal(&cloneCfg); err != nil {
		return fmt.Errorf("unable to unmarshal config: %w", err)
	}

	cfg.PrometheusConfig = cloneCfg.PrometheusConfig
	return nil
}

// If ecs service discovery is configured, this creates an empty service discovery file
func (t *translator) createEmptyEcsSdResultFile(conf *confmap.Conf) error {
	ecsSdPathKey := common.ConfigKey(prometheusKey, "ecs_service_discovery")
	if !conf.IsSet(ecsSdPathKey) {
		return nil
	}
	_, result := new(ecsservicediscovery.SDResultFile).ApplyRule(conf.Get(ecsSdPathKey)) // TODO: remove dependency on rule.
	ecsSdPath, ok := result.(string)
	if !ok || result == "" {
		return fmt.Errorf("unable to extract ecs service discovery result file path")
	}
	_, err := os.Stat(ecsSdPath)
	if os.IsNotExist(err) {
		file, err := os.Create(ecsSdPath)
		if err != nil {
			return err
		}
		err = file.Close()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

// If ecs service discovery is configured, add ecs relabel rules to maintain backwards compatibility
func (t *translator) addEcsRelabelConfigs(conf *confmap.Conf, cfg *prometheusreceiver.Config) error {
	ecsSdPathKey := common.ConfigKey(prometheusKey, "ecs_service_discovery")
	if !conf.IsSet(ecsSdPathKey) {
		return nil
	}
	for _, sc := range cfg.PrometheusConfig.ScrapeConfigs {
		// TODO: handle for edge case when customer themselves have relabel configs for these ecs labels
		sc.RelabelConfigs = append(sc.RelabelConfigs, EcsRelabelConfigs...)
		sc.MetricRelabelConfigs = append(sc.MetricRelabelConfigs, EcsMetricRelabelConfigs...)
	}
	return nil
}

func (t *translator) copyPrometheusConfigToConfigPlaceholder(cfg *prometheusreceiver.Config) error {

	// "-" PrometheusConfig => https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/7f4d4425a03e7e47575211be489f912cd16ae509/receiver/prometheusreceiver/config.go#L52
	// "config" ConfigPlaceholder => https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/7f4d4425a03e7e47575211be489f912cd16ae509/receiver/prometheusreceiver/config.go#L69

	// Since "-" PrometheusConfig represents the actual prometheus config which uses its own yaml tags & unmarshalling,
	// the OTel prometheus_receiver handles this by ignoring the PrometheusConfig field from mapstructure encoding/decoding
	// in combination with a dummy field "config" for ConfigPlaceholder.
	// It then uses some custom logic (https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/7f4d4425a03e7e47575211be489f912cd16ae509/receiver/prometheusreceiver/config.go#L258)
	// to first take the mapstructure-unmarshalled "config" ConfigPlaceholder and marshal that into an
	// intermediary yaml. This yaml is then unmarshalled into the actual "-" PrometheusConfig respecting the yaml tags & rules.

	// When it comes to our code, when we eventually TranslateJsonMapToYamlConfig, we use mapstructure encoder meaning the
	// actual "-" PrometheusConfig will be skipped and only the "config" ConfigPlaceholder will translate into the output yaml.
	// Hence, the very last step of prometheus translation should be to copy over PrometheusConfig to ConfigPlaceholder.
	// One could think why not just use ConfigPlaceholder everywhere in our translation steps without ever touching
	// PrometheusConfig but that isn't type safe and will result in a lot of unreadable code.

	// Another thing to note - Given we use mapstructure encoder, if we directly just copy over the PrometheusConfig
	// into ConfigPlaceholder, it would not respect the omitempty & other tags (since they are yaml tags).
	// Hence, we need to do something similar to the OTel prometheus_receiver where first we marshall into an intermediary
	// stream respecting the actual PrometheusConfig's yaml tags & rules and finally unmarshall this back into ConfigPlaceholder.

	prometheusConfig, err := yaml.Marshal(cfg.PrometheusConfig)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(prometheusConfig, &cfg.ConfigPlaceholder)
	if err != nil {
		return err
	}
	return nil
}
