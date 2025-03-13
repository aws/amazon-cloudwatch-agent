package awsebsnvme

import (
	"time"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
)

var (
	baseKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.DiskIOKey)
)

const (
	defaultCollectionInterval = time.Minute
)

type translator struct {
	common.NameProvider
	factory receiver.Factory
}

func NewTranslator(
	opts ...common.TranslatorOption,
) common.ComponentTranslator {
	t := &translator{factory: awsebsnvmereceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*awsebsnvmereceiver.Config)

	agentIntervalChain := []string{
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, agentIntervalChain, defaultCollectionInterval)
	if collectionInterval, ok := common.GetString(conf, common.ConfigKey(baseKey, common.MetricsCollectionIntervalKey)); ok {
		if dur, err := common.ParseDuration(collectionInterval); err == nil {
			cfg.CollectionInterval = dur
		}
	}

	// DOMINIC: respect the `resources` field under diskio
	// DOMINIC: need to add the cluster name from env vars

	return cfg, nil
}
