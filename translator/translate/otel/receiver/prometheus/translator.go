package prometheus

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	otelConfigParsingError = "has invalid keys: global"
	defaultTlsCaPath       = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	defaultScrapeProtocol  = "http" // Add default scrape protocol
)

var (
	configPathKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.PrometheusKey, common.PrometheusConfigPathKey)
)

type translator struct {
	name    string
	factory receiver.Factory
}

type Option func(any)

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...Option) common.ComponentTranslator {
	t := &translator{factory: prometheusreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*prometheusreceiver.Config)

	if !conf.IsSet(configPathKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configPathKey}
	}

	configPath, _ := common.GetString(conf, configPathKey)
	processedConfigPath, err := util.GetConfigPath("prometheus.yaml", configPathKey, configPath, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to process prometheus config with given config: %w", err)
	}
	configPath = processedConfigPath.(string)
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read prometheus config from path: %w", err)
	}

	var stringMap map[string]interface{}
	err = yaml.Unmarshal(content, &stringMap)
	if err != nil {
		return nil, err
	}

	// Add metric type hints through scrape_config
	if scrapeConfigs, ok := stringMap["scrape_configs"].([]interface{}); ok {
		for _, sc := range scrapeConfigs {
			if config, ok := sc.(map[string]interface{}); ok {
				// Add metric relabel configs if not present
				if _, exists := config["metric_relabel_configs"]; !exists {
					config["metric_relabel_configs"] = []map[string]interface{}{
						{
							"source_labels": []string{"__name__"},
							"regex":         "^.*_total$",
							"target_label":  "__type__",
							"replacement":   "counter",
						},
						{
							"source_labels": []string{"__name__"},
							"regex":         "^.*_(sum|count)$",
							"target_label":  "__type__",
							"replacement":   "counter",
						},
						{
							"source_labels": []string{"__name__"},
							"regex":         "^.*_bucket$",
							"target_label":  "__type__",
							"replacement":   "histogram",
						},
					}
				}
			}
		}
	}

	componentParser := confmap.NewFromStringMap(stringMap)
	if componentParser == nil {
		return nil, fmt.Errorf("unable to parse config from filename %s", configPath)
	}

	err = componentParser.Unmarshal(&cfg)
	if err != nil {
		// Handle plain prometheus format
		if !strings.Contains(err.Error(), otelConfigParsingError) {
			return nil, fmt.Errorf("unable to unmarshall config to otel prometheus config from filename %s", configPath)
		}

		var promCfg prometheusreceiver.PromConfig
		err = componentParser.Unmarshal(&promCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall config to prometheus config from filename %s", configPath)
		}

		cfg.PrometheusConfig.GlobalConfig = promCfg.GlobalConfig
		cfg.PrometheusConfig.ScrapeConfigs = promCfg.ScrapeConfigs
		cfg.PrometheusConfig.TracingConfig = promCfg.TracingConfig
	} else {
		// Handle OTel format
		if cfg.TargetAllocator != nil && len(cfg.TargetAllocator.CollectorID) > 0 {
			cfg.TargetAllocator.TLSSetting.Config.CAFile = defaultTlsCaPath
			cfg.TargetAllocator.TLSSetting.ReloadInterval = 10 * time.Second
		}
	}

	return cfg, nil
}
