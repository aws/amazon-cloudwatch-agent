// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscontainerinsight

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	globallogs "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	logsutil "github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

// container orchestrator keys
const (
	ecs = "ecs"
	eks = "eks"

	defaultMetricsCollectionInterval = time.Minute
	defaultLeaderLockName            = "cwagent-clusterleader" // To maintain backwards compatibility with https://github.com/aws/amazon-cloudwatch-agent/blob/2dd89abaab4590cffbbc31ef89319b62809b09d1/plugins/inputs/k8sapiserver/k8sapiserver.go#L30
)

type translator struct {
	name    string
	factory receiver.Factory
	// services is a slice of config keys to orchestrators.
	services []*collections.Pair[string, string]
}

var _ common.Translator[component.Config] = (*translator)(nil)

// NewTranslator creates a new aws container insight receiver translator.
func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	baseKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey)
	return &translator{
		name:    name,
		factory: awscontainerinsightreceiver.NewFactory(),
		services: []*collections.Pair[string, string]{
			{Key: common.ConfigKey(baseKey, common.ECSKey), Value: ecs},
			{Key: common.ConfigKey(baseKey, common.KubernetesKey), Value: eks},
		},
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an aws container insights receiver config if either
// of the sections defined in the services exist.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	configuredService := t.getConfiguredContainerService(conf)
	if configuredService == nil {
		var keys []string
		for _, service := range t.services {
			keys = append(keys, service.Key)
		}
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: strings.Join(keys, " or ")}
	}
	cfg := t.factory.CreateDefaultConfig().(*awscontainerinsightreceiver.Config)
	intervalKeyChain := []string{
		common.ConfigKey(configuredService.Key, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, defaultMetricsCollectionInterval)
	cfg.ContainerOrchestrator = configuredService.Value
	cfg.AWSSessionSettings.Region = agent.Global_Config.Region
	if profileKey, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.AWSSessionSettings.Profile = fmt.Sprintf("%v", profileKey)
	}
	if credentialsFileKey, ok := agent.Global_Config.Credentials[agent.CredentialsFile_Key]; ok {
		cfg.AWSSessionSettings.SharedCredentialsFile = []string{fmt.Sprintf("%v", credentialsFileKey)}
	}
	cfg.AWSSessionSettings.IMDSRetries = retryer.GetDefaultRetryNumber()

	if configuredService.Value == eks {
		if err := t.setClusterName(conf, cfg); err != nil {
			return nil, err
		}
		cfg.LeaderLockName = defaultLeaderLockName
		cfg.LeaderLockUsingConfigMapOnly = true
		tagServiceKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, "tag_service")
		cfg.TagService = common.GetOrDefaultBool(conf, tagServiceKey, true)

		if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
			cfg.LocalMode = true
		}

		if EnhancedContainerInsightsEnabled(conf) {
			cfg.AddFullPodNameMetricLabel = true
			cfg.AddContainerNameMetricLabel = true
			cfg.PrefFullPodName = true
			cfg.EnableControlPlaneMetrics = true
		}

		if kubeConfigPath, ok := common.GetString(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, "kube_config_path")); ok {
			cfg.KubeConfigPath = kubeConfigPath
		}

		t.setHostName(conf, cfg)
		t.setHostIP(conf, cfg)
		cfg.RunOnSystemd = !context.CurrentContext().RunInContainer()
	}

	cfg.PrefFullPodName = cfg.PrefFullPodName || common.GetOrDefaultBool(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.PreferFullPodName), false)
	cfg.EnableAcceleratedComputeMetrics = cfg.EnableAcceleratedComputeMetrics || AcceleratedComputeMetricsEnabled(conf)

	return cfg, nil
}

func (t *translator) setClusterName(conf *confmap.Conf, cfg *awscontainerinsightreceiver.Config) error {
	clusterNameKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, "cluster_name")
	if clusterName, ok := common.GetString(conf, clusterNameKey); ok {
		cfg.ClusterName = clusterName
	} else {
		cfg.ClusterName = util.GetClusterNameFromEc2Tagger()
	}

	if cfg.ClusterName == "" {
		return errors.New("cluster name is not provided and was not auto-detected from EC2 tags")
	}
	return nil
}

func (t *translator) setHostIP(conf *confmap.Conf, cfg *awscontainerinsightreceiver.Config) {
	hostIPKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, "host_ip")
	if hostIP, ok := common.GetString(conf, hostIPKey); ok {
		cfg.HostIP = logsutil.ResolvePlaceholder(hostIP, globallogs.GlobalLogConfig.MetadataInfo)
	}
}

func (t *translator) setHostName(conf *confmap.Conf, cfg *awscontainerinsightreceiver.Config) {
	hostNameKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, "host_name")
	if hostName, ok := common.GetString(conf, hostNameKey); ok {
		cfg.HostName = logsutil.ResolvePlaceholder(hostName, globallogs.GlobalLogConfig.MetadataInfo)
	}
}

// getConfiguredContainerService gets the first found container service
// from the service slice.
func (t *translator) getConfiguredContainerService(conf *confmap.Conf) *collections.Pair[string, string] {
	var configuredService *collections.Pair[string, string]
	if conf != nil {
		for _, service := range t.services {
			if conf.IsSet(service.Key) {
				configuredService = service
				break
			}
		}
	}
	return configuredService
}
