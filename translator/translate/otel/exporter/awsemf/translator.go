// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsight"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

//go:embed awsemf_default_ecs.yaml
var defaultEcsConfig string

//go:embed awsemf_default_kubernetes.yaml
var defaultKubernetesConfig string

//go:embed awsemf_default_prometheus.yaml
var defaultPrometheusConfig string

//go:embed appsignals_config_eks.yaml
var appSignalsConfigEks string

//go:embed appsignals_config_k8s.yaml
var appSignalsConfigK8s string

//go:embed appsignals_config_generic.yaml
var appSignalsConfigGeneric string

var (
	ecsBasePathKey          = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.ECSKey)
	kubernetesBasePathKey   = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey)
	prometheusBasePathKey   = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
	emfProcessorBasePathKey = common.ConfigKey(prometheusBasePathKey, common.EMFProcessorKey)
	endpointOverrideKey     = common.ConfigKey(common.LogsKey, common.EndpointOverrideKey)
	roleARNPathKey          = common.ConfigKey(common.LogsKey, common.CredentialsKey, common.RoleARNKey)
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
	cfg.MiddlewareID = &agenthealth.LogsID

	var defaultConfig string
	if t.isAppSignals(c) {
		defaultConfig = getAppSignalsConfig()
	} else if isEcs(c) {
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
	cfg.AWSSessionSettings.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	if c.IsSet(endpointOverrideKey) {
		cfg.AWSSessionSettings.Endpoint, _ = common.GetString(c, endpointOverrideKey)
	}
	cfg.AWSSessionSettings.IMDSRetries = retryer.GetDefaultRetryNumber()
	if profileKey, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.AWSSessionSettings.Profile = fmt.Sprintf("%v", profileKey)
	}
	cfg.AWSSessionSettings.Region = agent.Global_Config.Region
	cfg.AWSSessionSettings.RoleARN = agent.Global_Config.Role_arn
	if c.IsSet(roleARNPathKey) {
		cfg.AWSSessionSettings.RoleARN, _ = common.GetString(c, roleARNPathKey)
	}
	if credentialsFileKey, ok := agent.Global_Config.Credentials[agent.CredentialsFile_Key]; ok {
		cfg.AWSSessionSettings.SharedCredentialsFile = []string{fmt.Sprintf("%v", credentialsFileKey)}
	}
	if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
		cfg.AWSSessionSettings.LocalMode = true
	}

	if t.isAppSignals(c) {
		if err := setAppSignalsFields(c, cfg); err != nil {
			return nil, err
		}
	} else if isEcs(c) {
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

func getAppSignalsConfig() string {
	ctx := context.CurrentContext()

	mode := ctx.KubernetesMode()
	if mode == "" {
		mode = ctx.Mode()
	}
	if mode == config.ModeEC2 {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			mode = config.ModeECS
		}
	}

	switch mode {
	case config.ModeEKS:
		return appSignalsConfigEks
	case config.ModeK8sEC2, config.ModeK8sOnPrem:
		return appSignalsConfigK8s
	case config.ModeEC2, config.ModeECS:
		return appSignalsConfigGeneric
	default:
		return appSignalsConfigGeneric
	}
}

func (t *translator) isAppSignals(conf *confmap.Conf) bool {
	return (t.name == common.AppSignals || t.name == common.AppSignalsFallback) && (conf.IsSet(common.AppSignalsMetrics) || conf.IsSet(common.AppSignalsTraces) || conf.IsSet(common.AppSignalsMetricsFallback) || conf.IsSet(common.AppSignalsTracesFallback))
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

func setAppSignalsFields(_ *confmap.Conf, _ *awsemfexporter.Config) error {
	return nil
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

	if awscontainerinsight.EnhancedContainerInsightsEnabled(conf) {
		cfg.EnhancedContainerInsights = true
	}

	return nil
}

func setPrometheusFields(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	setDisableMetricExtraction(prometheusBasePathKey, conf, cfg)

	if err := setPrometheusLogGroup(conf, cfg); err != nil {
		return err
	}

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
