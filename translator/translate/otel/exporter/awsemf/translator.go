// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	_ "embed"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

//go:embed awsemf_default_ecs.yaml
var defaultEcsConfig string

//go:embed awsemf_default_kubernetes.yaml
var defaultKubernetesConfig string

//go:embed awsemf_default_prometheus.yaml
var defaultPrometheusConfig string

var (
	ecsBasePathKey          = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.ECSKey)
	kubernetesBasePathKey   = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey)
	prometheusBasePathKey   = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
	emfProcessorBasePathKey = common.ConfigKey(prometheusBasePathKey, common.EMFProcessorKey)
)

type translator struct {
	name    string
	factory exporter.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, awsemfexporter.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an awsemf exporter config based on the input json config
func (t *translator) Translate(c *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awsemfexporter.Config)

	var defaultConfig string
	if isEcs(c) {
		defaultConfig = defaultEcsConfig
	} else if isKubernetes(c) {
		defaultConfig = defaultKubernetesConfig
	} else if isPrometheus(c) {
		defaultConfig = defaultPrometheusConfig
	} else {
		return cfg, nil
	}
	if defaultConfig != "" {
		var rawConf map[string]interface{}
		if err := yaml.Unmarshal([]byte(defaultConfig), &rawConf); err != nil {
			return nil, fmt.Errorf("unable to read default config: %w", err)
		}
		conf := confmap.NewFromStringMap(rawConf)
		if err := conf.Unmarshal(&cfg); err != nil {
			return nil, fmt.Errorf("unable to unmarshal config: %w", err)
		}
	}
	cfg.AWSSessionSettings.Region = agent.Global_Config.Region
	if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
		if profile, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
			cfg.AWSSessionSettings.Profile = fmt.Sprintf("%v", profile)
			cfg.AWSSessionSettings.SharedCredentialsFile = []string{fmt.Sprintf("%v", agent.Global_Config.Credentials[agent.CredentialsFile_Key])}
		}
	}

	if isEcs(c) {
		if err := setEcsFields(c, cfg); err != nil {
			return nil, err
		}
	} else if isKubernetes(c) {
		if err := setKubernetesFields(c, cfg); err != nil {
			return nil, err
		}
	} else if isPrometheus(c) {
		if err := setPrometheusFields(c, cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func isEcs(conf *confmap.Conf) bool {
	return conf.IsSet(ecsBasePathKey)
}

func isKubernetes(conf *confmap.Conf) bool {
	return conf.IsSet(kubernetesBasePathKey)
}

func isPrometheus(conf *confmap.Conf) bool {
	return conf.IsSet(prometheusBasePathKey)
}

func setEcsFields(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	setDisableMetricExtraction(ecsBasePathKey, conf, cfg)
	return nil
}

func setKubernetesFields(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	setDisableMetricExtraction(kubernetesBasePathKey, conf, cfg)

	if err := setKubernetesMetricDeclaration(conf, cfg); err != nil {
		return err
	}
	return nil
}

func setPrometheusFields(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	setDisableMetricExtraction(prometheusBasePathKey, conf, cfg)

	if err := setPrometheusLogGroup(conf, cfg); err != nil {
		return err
	}

	// Prometheus will use the "job" corresponding to the target in prometheus as a log stream
	// https://github.com/aws/amazon-cloudwatch-agent/blob/59cfe656152e31ca27e7983fac4682d0c33d3316/plugins/inputs/prometheus_scraper/metrics_handler.go#L80-L84
	// While determining the target, we would give preference to the metric tag over the log_stream_name coming from config/toml as per
	// https://github.com/aws/amazon-cloudwatch-agent/blob/main/plugins/outputs/cloudwatchlogs/cloudwatchlogs.go#L176-L181.

	// However, since we are using awsemfexport, we can leverage the token replacement with the log stream name
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/897db04f747f0bda1707c916b1ec9f6c79a0c678/exporter/awsemfexporter/util.go#L29-L37
	// Therefore, add a tag {ServiceName} for replacing job as a log stream

	if conf.IsSet(emfProcessorBasePathKey) {
		if err := setPrometheusNamespace(conf, cfg); err != nil {
			return err
		}
		if err := setPrometheusMetricDescriptors(conf, cfg); err != nil {
			return err
		}
		if err := setPrometheusMetricDeclarations(conf, cfg); err != nil {
			return err
		}
	}

	if len(cfg.MetricDeclarations) == 0 {
		// When there are no metric declarations, CWA does not generate any EMF structured logs and instead just publishes them as plain log events
		// The awsemfexporter by default generates EMF structured logs for all if there are no metric declarations, hence adding a dummy rule here to prevent it
		cfg.MetricDeclarations = []*awsemfexporter.MetricDeclaration{
			{
				MetricNameSelectors: []string{"$^"},
			},
		}
	}
	return nil
}

func setDisableMetricExtraction(baseKey string, conf *confmap.Conf, cfg *awsemfexporter.Config) {
	cfg.DisableMetricExtraction = common.GetOrDefaultBool(conf, common.ConfigKey(baseKey, common.DisableMetricExtraction), false)
}
