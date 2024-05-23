// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"container/list"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"
)

const (
	AgentKey                           = "agent"
	DebugKey                           = "debug"
	MetricsKey                         = "metrics"
	LogsKey                            = "logs"
	TracesKey                          = "traces"
	MetricsCollectedKey                = "metrics_collected"
	LogsCollectedKey                   = "logs_collected"
	TracesCollectedKey                 = "traces_collected"
	ECSKey                             = "ecs"
	KubernetesKey                      = "kubernetes"
	PrometheusKey                      = "prometheus"
	EMFProcessorKey                    = "emf_processor"
	DisableMetricExtraction            = "disable_metric_extraction"
	XrayKey                            = "xray"
	OtlpKey                            = "otlp"
	EndpointOverrideKey                = "endpoint_override"
	RegionOverrideKey                  = "region_override"
	ProxyOverrideKey                   = "proxy_override"
	InsecureKey                        = "insecure"
	LocalModeKey                       = "local_mode"
	CredentialsKey                     = "credentials"
	RoleARNKey                         = "role_arn"
	MetricsCollectionIntervalKey       = "metrics_collection_interval"
	MeasurementKey                     = "measurement"
	DropOriginalMetricsKey             = "drop_original_metrics"
	ForceFlushIntervalKey              = "force_flush_interval"
	ContainerInsightsMetricGranularity = "metric_granularity" // replaced with enhanced_container_insights
	EnhancedContainerInsights          = "enhanced_container_insights"
	PreferFullPodName                  = "prefer_full_pod_name"
	EnableAcceleratedComputeMetric     = "accelerated_compute_metrics"
	Console                            = "console"
	DiskKey                            = "disk"
	DiskIOKey                          = "diskio"
	NetKey                             = "net"
	Emf                                = "emf"
	StructuredLog                      = "structuredlog"
	ServiceAddress                     = "service_address"
	Udp                                = "udp"
	Tcp                                = "tcp"
	TlsKey                             = "tls"
	Region                             = "region"
	LogGroupName                       = "log_group_name"
	LogStreamName                      = "log_stream_name"
)

const (
	PipelineNameHost             = "host"
	PipelineNameHostDeltaMetrics = "hostDeltaMetrics"
	PipelineNameEmfLogs          = "emf_logs"
	AppSignals                   = "application_signals"
	AppSignalsFallback           = "app_signals"
	AppSignalsRules              = "rules"
)

var (
	AppSignalsTraces          = ConfigKey(TracesKey, TracesCollectedKey, AppSignals)
	AppSignalsMetrics         = ConfigKey(LogsKey, MetricsCollectedKey, AppSignals)
	AppSignalsTracesFallback  = ConfigKey(TracesKey, TracesCollectedKey, AppSignalsFallback)
	AppSignalsMetricsFallback = ConfigKey(LogsKey, MetricsCollectedKey, AppSignalsFallback)

	AppSignalsConfigKeys = map[component.DataType][]string{
		component.DataTypeTraces:  {AppSignalsTraces, AppSignalsTracesFallback},
		component.DataTypeMetrics: {AppSignalsMetrics, AppSignalsMetricsFallback},
	}

	AgentDebugConfigKey = ConfigKey(AgentKey, DebugKey)
)

// Translator is used to translate the JSON config into an
// OTEL config.
type Translator[C any] interface {
	Translate(*confmap.Conf) (C, error)
	ID() component.ID
}

// TranslatorMap is a set of translators by their types.
type TranslatorMap[C any] interface {
	// Set a translator to the map. If the ID is already present, replaces the translator.
	// Otherwise, adds it to the end of the list.
	Set(Translator[C])
	// Get the translator for the component.ID.
	Get(component.ID) (Translator[C], bool)
	// Merge another translator map in.
	Merge(TranslatorMap[C])
	// Keys is the ordered component.IDs.
	Keys() []component.ID
	// Range iterates over each translator in order and calls the callback function on each.
	Range(func(Translator[C]))
	// Len is the number of translators in the map.
	Len() int
}

type translatorMap[C any] struct {
	// list stores the ordered translators.
	list *list.List
	// lookup stores the list.Elements containing the translators by ID.
	lookup map[component.ID]*list.Element
}

func (t translatorMap[C]) Set(translator Translator[C]) {
	if element, ok := t.lookup[translator.ID()]; ok {
		element.Value = translator
	} else {
		element = t.list.PushBack(translator)
		t.lookup[translator.ID()] = element
	}
}

func (t translatorMap[C]) Get(id component.ID) (Translator[C], bool) {
	element, ok := t.lookup[id]
	return element.Value.(Translator[C]), ok
}

func (t translatorMap[C]) Merge(other TranslatorMap[C]) {
	if other != nil {
		other.Range(t.Set)
	}
}

func (t translatorMap[C]) Keys() []component.ID {
	keys := make([]component.ID, 0, t.Len())
	t.Range(func(translator Translator[C]) {
		keys = append(keys, translator.ID())
	})
	return keys
}

func (t translatorMap[C]) Range(callback func(translator Translator[C])) {
	for element := t.list.Front(); element != nil; element = element.Next() {
		callback(element.Value.(Translator[C]))
	}
}

func (t translatorMap[C]) Len() int {
	return t.list.Len()
}

// NewTranslatorMap creates a TranslatorMap from the translators.
func NewTranslatorMap[C any](translators ...Translator[C]) TranslatorMap[C] {
	t := translatorMap[C]{
		list:   list.New(),
		lookup: make(map[component.ID]*list.Element, len(translators)),
	}
	for _, translator := range translators {
		t.Set(translator)
	}
	return t
}

// A MissingKeyError occurs when a translator is used for a JSON
// config that does not have a required key. This typically means
// that the pipeline was configured incorrectly.
type MissingKeyError struct {
	ID      component.ID
	JsonKey string
}

func (e *MissingKeyError) Error() string {
	return fmt.Sprintf("%q missing key in JSON: %q", e.ID, e.JsonKey)
}

// ComponentTranslators is a component ID and respective service pipeline.
type ComponentTranslators struct {
	Receivers  TranslatorMap[component.Config]
	Processors TranslatorMap[component.Config]
	Exporters  TranslatorMap[component.Config]
	Extensions TranslatorMap[component.Config]
}

// ConfigKey joins the keys separated by confmap.KeyDelimiter.
// This helps translators navigate the confmap.Conf that the
// JSON config is loaded into.
func ConfigKey(keys ...string) string {
	return strings.Join(keys, confmap.KeyDelimiter)
}

// ParseDuration attempts to parse the input into a duration.
// Returns a zero duration and an error if invalid.
func ParseDuration(v interface{}) (time.Duration, error) {
	if v != nil {
		if fv, ok := v.(float64); ok {
			return time.Second * time.Duration(fv), nil
		}
		s, ok := v.(string)
		if !ok {
			s = fmt.Sprintf("%v", v)
		}
		duration, err := time.ParseDuration(s)
		if err == nil {
			return duration, nil
		}
		sI, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			return time.Second * time.Duration(sI), nil
		}
		sF, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return time.Second * time.Duration(sF), nil
		}
	}
	return time.Duration(0), fmt.Errorf("invalid type %v", reflect.TypeOf(v))
}

// GetString gets the string value for the key. If the key is missing,
// ok will be false.
func GetString(conf *confmap.Conf, key string) (string, bool) {
	if value := conf.Get(key); value != nil {
		got, ok := value.(string)
		// if the value isn't a string, convert it
		if !ok {
			got = fmt.Sprintf("%v", value)
			ok = true
		}
		return got, ok
	}
	return "", false
}

// GetArray gets the array value for the key. If the key is missing,
// the return value will be nil
func GetArray[C any](conf *confmap.Conf, key string) []C {
	if value := conf.Get(key); value != nil {
		var arr []C
		got, _ := value.([]any)
		for _, entry := range got {
			if t, ok := entry.(C); ok {
				arr = append(arr, t)
			}
		}
		return arr
	}
	return nil
}

// GetBool gets the bool value for the key. If the key is missing or the
// value is not a bool type, then ok will be false.
func GetBool(conf *confmap.Conf, key string) (value bool, ok bool) {
	if v := conf.Get(key); v != nil {
		value, ok = v.(bool)
	}
	return
}

// GetOrDefaultBool gets the bool value for the key. If the key is missing or the
// value is not a bool type, then the defaultVal is returned.
func GetOrDefaultBool(conf *confmap.Conf, key string, defaultVal bool) bool {
	if v := conf.Get(key); v != nil {
		if val, ok := v.(bool); ok {
			return val
		}
	}
	return defaultVal
}

// GetNumber gets the number value for the key. The switch works through
// all reasonable number types (the default is typically float64)
func GetNumber(conf *confmap.Conf, key string) (float64, bool) {
	if v := conf.Get(key); v != nil {
		switch i := v.(type) {
		case float64:
			return i, true
		case float32:
			return float64(i), true
		case int64:
			return float64(i), true
		case int32:
			return float64(i), true
		case int:
			return float64(i), true
		case uint64:
			return float64(i), true
		case uint32:
			return float64(i), true
		case uint:
			return float64(i), true
		case string:
		}
	}
	return 0, false
}

// GetOrDefaultNumber gets the number value for the key. If the key is missing or the
// value is not a number type, then the defaultVal is returned.
func GetOrDefaultNumber(conf *confmap.Conf, key string, defaultVal float64) float64 {
	value, ok := GetNumber(conf, key)
	if !ok {
		return defaultVal
	}
	return value
}

// GetDuration gets the value for the key and calls ParseDuration on it.
// If the key is missing, it is unable to parse the duration, or the
// duration is set to 0, then the returned bool will be false.
func GetDuration(conf *confmap.Conf, key string) (time.Duration, bool) {
	var duration time.Duration
	var ok bool
	if value := conf.Get(key); value != nil {
		var err error
		duration, err = ParseDuration(value)
		ok = err == nil && duration > 0
	}
	return duration, ok
}

// GetOrDefaultDuration from the first section in the keychain with a
// parsable duration. If none are found, returns the defaultDuration.
func GetOrDefaultDuration(conf *confmap.Conf, keychain []string, defaultDuration time.Duration) time.Duration {
	for _, key := range keychain {
		duration, ok := GetDuration(conf, key)
		if !ok {
			continue
		}
		return duration
	}
	return defaultDuration
}

func GetYamlFileToYamlConfig(cfg interface{}, yamlFile string) (interface{}, error) {
	var cfgMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlFile), &cfgMap); err != nil {
		return nil, fmt.Errorf("unable to read default config: %w", err)
	}

	conf := confmap.NewFromStringMap(cfgMap)
	if err := conf.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}
	return cfg, nil
}
