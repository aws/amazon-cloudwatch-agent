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
	defaultScrapeProtocol  = "PrometheusText0.0.4"
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

func WithName(name string) Option {
	return func(a any) {
		if t, ok := a.(*translator); ok {
			t.name = name
		}
	}
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

	// Add global config if not present
	if _, exists := stringMap["global"]; !exists {
		stringMap["global"] = map[string]interface{}{
			"metric_name_validation_scheme": "utf8",
			"scrape_interval":               "1m",
			"scrape_timeout":                "10s",
			"evaluation_interval":           "1m",
			"scrape_protocols": []string{
				"OpenMetricsText1.0.0",
				"PrometheusText0.0.4",
			},
		}
	}

	// Update scrape configs with Prometheus 3.0 requirements
	if scrapeConfigs, ok := stringMap["scrape_configs"].([]interface{}); ok {
		for i, sc := range scrapeConfigs {
			if config, ok := sc.(map[string]interface{}); ok {
				// Add fallback scrape protocol if not present
				if _, exists := config["fallback_scrape_protocol"]; !exists {
					config["fallback_scrape_protocol"] = defaultScrapeProtocol
				}

				// Add scrape protocols if not present
				if _, exists := config["scrape_protocols"]; !exists {
					config["scrape_protocols"] = []string{
						"OpenMetricsText1.0.0",
						"PrometheusText0.0.4",
					}
				}

				// Add always_scrape_classic_histograms
				if _, exists := config["always_scrape_classic_histograms"]; !exists {
					config["always_scrape_classic_histograms"] = true
				}

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

				// Validate the scrape config
				if err := validateScrapeConfig(config); err != nil {
					return nil, fmt.Errorf("invalid scrape config at index %d: %w", i, err)
				}

				scrapeConfigs[i] = config
			}
		}
		stringMap["scrape_configs"] = scrapeConfigs
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

func validateScrapeConfig(config map[string]interface{}) error {
	// Validate scrape protocols
	if protocols, ok := config["scrape_protocols"].([]string); ok {
		for _, protocol := range protocols {
			switch protocol {
			case "PrometheusText0.0.4", "OpenMetricsText1.0.0", "PrometheusProto":
				continue
			default:
				return fmt.Errorf("unsupported scrape protocol: %s", protocol)
			}
		}
	}

	// Validate fallback protocol
	if fallback, ok := config["fallback_scrape_protocol"].(string); ok {
		switch fallback {
		case "PrometheusText0.0.4", "OpenMetricsText1.0.0", "PrometheusProto":
			// valid
		default:
			return fmt.Errorf("unsupported fallback scrape protocol: %s", fallback)
		}
	}

	return nil
}
