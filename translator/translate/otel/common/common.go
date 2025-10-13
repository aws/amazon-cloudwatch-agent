// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"container/list"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
)

const (
	AgentKey                                       = "agent"
	DebugKey                                       = "debug"
	MetricsKey                                     = "metrics"
	LogsKey                                        = "logs"
	TracesKey                                      = "traces"
	MetricsCollectedKey                            = "metrics_collected"
	LogsCollectedKey                               = "logs_collected"
	TracesCollectedKey                             = "traces_collected"
	MetricsDestinationsKey                         = "metrics_destinations"
	ECSKey                                         = "ecs"
	KubernetesKey                                  = "kubernetes"
	CloudWatchKey                                  = "cloudwatch"
	CloudWatchLogsKey                              = "cloudwatchlogs"
	PrometheusKey                                  = "prometheus"
	PrometheusConfigPathKey                        = "prometheus_config_path"
	AMPKey                                         = "amp"
	WorkspaceIDKey                                 = "workspace_id"
	EMFProcessorKey                                = "emf_processor"
	DisableMetricExtraction                        = "disable_metric_extraction"
	XrayKey                                        = "xray"
	OtlpKey                                        = "otlp"
	JmxKey                                         = "jmx"
	TLSKey                                         = "tls"
	Endpoint                                       = "endpoint"
	EndpointOverrideKey                            = "endpoint_override"
	RegionOverrideKey                              = "region_override"
	ProxyOverrideKey                               = "proxy_override"
	InsecureKey                                    = "insecure"
	LocalModeKey                                   = "local_mode"
	CredentialsKey                                 = "credentials"
	RoleARNKey                                     = "role_arn"
	SigV4Auth                                      = "sigv4auth"
	MetricsCollectionIntervalKey                   = "metrics_collection_interval"
	AggregationDimensionsKey                       = "aggregation_dimensions"
	MeasurementKey                                 = "measurement"
	DropOriginalMetricsKey                         = "drop_original_metrics"
	ForceFlushIntervalKey                          = "force_flush_interval"
	ContainerInsightsMetricGranularity             = "metric_granularity" // replaced with enhanced_container_insights
	EnhancedContainerInsights                      = "enhanced_container_insights"
	ResourcesKey                                   = "resources"
	PreferFullPodName                              = "prefer_full_pod_name"
	EnableAcceleratedComputeMetric                 = "accelerated_compute_metrics"
	AcceleratedComputeGPUMetricsCollectionInterval = "accelerated_compute_gpu_metrics_collection_interval"
	HighFrequencyGpuMetrics                        = "high_frequency_gpu_metrics"
	EnableKueueContainerInsights                   = "kueue_container_insights"
	AppendDimensionsKey                            = "append_dimensions"
	Console                                        = "console"
	DiskKey                                        = "disk"
	DiskIOKey                                      = "diskio"
	NetKey                                         = "net"
	Emf                                            = "emf"
	StructuredLog                                  = "structuredlog"
	ServiceAddress                                 = "service_address"
	UDP                                            = "udp"
	TCP                                            = "tcp"
	TlsKey                                         = "tls" //nolint:revive
	Tags                                           = "tags"
	Region                                         = "region"
	LogGroupName                                   = "log_group_name"
	LogStreamName                                  = "log_stream_name"
	NameKey                                        = "name"
	RenameKey                                      = "rename"
	UnitKey                                        = "unit"
)

const (
	CollectDMetricKey = "collectd"
	CollectDPluginKey = "socket_listener"
	CPUMetricKey      = "cpu"
	DiskMetricKey     = "disk"
	DiskIoMetricKey   = "diskio"
	StatsDMetricKey   = "statsd"
	SwapMetricKey     = "swap"
	MemMetricKey      = "mem"
	NetMetricKey      = "net"
	NetStatMetricKey  = "netstat"
	ProcessMetricKey  = "process"
	ProcStatMetricKey = "procstat"

	//Windows Plugins
	MemMetricKeyWindows          = "Memory"
	LogicalDiskMetricKeyWindows  = "LogicalDisk"
	NetworkMetricKeyWindows      = "Network Interface"
	PagingMetricKeyWindows       = "Paging"
	PhysicalDiskMetricKeyWindows = "PhysicalDisk"
	ProcessorMetricKeyWindows    = "Processor"
	SystemMetricKeyWindows       = "System"
	TCPv4MetricKeyWindows        = "TCPv4"
	TCPv6MetricKeyWindows        = "TCPv6"
)

const (
	PipelineNameHost                 = "host"
	PipelineNameHostCustomMetrics    = "hostCustomMetrics"
	PipelineNameHostDeltaMetrics     = "hostDeltaMetrics"
	PipelineNameHostOtlpMetrics      = "hostOtlpMetrics"
	PipelineNameContainerInsights    = "containerinsights"
	PipelineNameJmx                  = "jmx"
	PipelineNameContainerInsightsJmx = "containerinsightsjmx"
	PipelineNameEmfLogs              = "emf_logs"
	PipelineNamePrometheus           = "prometheus"
	PipelineNameKueue                = "kueueContainerInsights"
	AppSignals                       = "application_signals"
	AppSignalsFallback               = "app_signals"
	AppSignalsRules                  = "rules"
)

const (
	DiskIOPrefix = "diskio_"
)

var (
	AppSignalsTraces          = ConfigKey(TracesKey, TracesCollectedKey, AppSignals)
	AppSignalsMetrics         = ConfigKey(LogsKey, MetricsCollectedKey, AppSignals)
	AppSignalsTracesFallback  = ConfigKey(TracesKey, TracesCollectedKey, AppSignalsFallback)
	AppSignalsMetricsFallback = ConfigKey(LogsKey, MetricsCollectedKey, AppSignalsFallback)

	AppSignalsConfigKeys = map[pipeline.Signal][]string{
		pipeline.SignalTraces:  {AppSignalsTraces, AppSignalsTracesFallback},
		pipeline.SignalMetrics: {AppSignalsMetrics, AppSignalsMetricsFallback},
	}
	JmxConfigKey               = ConfigKey(MetricsKey, MetricsCollectedKey, JmxKey)
	ContainerInsightsConfigKey = ConfigKey(LogsKey, MetricsCollectedKey, KubernetesKey)

	JmxTargets = []string{"activemq", "cassandra", "hbase", "hadoop", "jetty", "jvm", "kafka", "kafka-consumer", "kafka-producer", "solr", "tomcat", "wildfly"}

	AgentDebugConfigKey             = ConfigKey(AgentKey, DebugKey)
	MetricsAggregationDimensionsKey = ConfigKey(MetricsKey, AggregationDimensionsKey)
	OTLPLogsKey                     = ConfigKey(LogsKey, MetricsCollectedKey, OtlpKey)
	OTLPMetricsKey                  = ConfigKey(MetricsKey, MetricsCollectedKey, OtlpKey)
)

type TranslatorID interface {
	component.ID | pipeline.ID

	Name() string
}

// Translator is used to translate the JSON config into an
// OTEL config.
type Translator[C any, ID TranslatorID] interface {
	Translate(*confmap.Conf) (C, error)
	ID() ID
}

// TranslatorMap is a set of translators by their types.
type TranslatorMap[C any, ID TranslatorID] interface {
	// Set a translator to the map. If the ID is already present, replaces the translator.
	// Otherwise, adds it to the end of the list.
	Set(Translator[C, ID])
	// Get the translator for the component.ID.
	Get(ID) (Translator[C, ID], bool)
	// Merge another translator map in.
	Merge(TranslatorMap[C, ID])
	// Keys is the ordered component.IDs.
	Keys() []ID
	// Range iterates over each translator in order and calls the callback function on each.
	Range(func(Translator[C, ID]))
	// Len is the number of translators in the map.
	Len() int
}

type translatorMap[C any, ID TranslatorID] struct {
	// list stores the ordered translators.
	list *list.List
	// lookup stores the list.Elements containing the translators by ID.
	lookup map[ID]*list.Element
}

func (t translatorMap[C, ID]) Set(translator Translator[C, ID]) {
	if element, ok := t.lookup[translator.ID()]; ok {
		element.Value = translator
	} else {
		element = t.list.PushBack(translator)
		t.lookup[translator.ID()] = element
	}
}

func (t translatorMap[C, ID]) Get(id ID) (Translator[C, ID], bool) {
	element, ok := t.lookup[id]
	if !ok {
		return nil, ok
	}
	return element.Value.(Translator[C, ID]), ok
}

func (t translatorMap[C, ID]) Merge(other TranslatorMap[C, ID]) {
	if other != nil {
		other.Range(t.Set)
	}
}

func (t translatorMap[C, ID]) Keys() []ID {
	keys := make([]ID, 0, t.Len())
	t.Range(func(translator Translator[C, ID]) {
		keys = append(keys, translator.ID())
	})
	return keys
}

func (t translatorMap[C, ID]) Range(callback func(translator Translator[C, ID])) {
	for element := t.list.Front(); element != nil; element = element.Next() {
		callback(element.Value.(Translator[C, ID]))
	}
}

func (t translatorMap[C, ID]) Len() int {
	return t.list.Len()
}

// NewTranslatorMap creates a TranslatorMap from the translators.
func NewTranslatorMap[C any, ID TranslatorID](translators ...Translator[C, ID]) TranslatorMap[C, ID] {
	t := translatorMap[C, ID]{
		list:   list.New(),
		lookup: make(map[ID]*list.Element, len(translators)),
	}
	for _, translator := range translators {
		t.Set(translator)
	}
	return t
}

type ID interface {
	String() string
}

// A MissingKeyError occurs when a translator is used for a JSON
// config that does not have a required key. This typically means
// that the pipeline was configured incorrectly.
type MissingKeyError struct {
	ID      ID
	JsonKey string
}

func (e *MissingKeyError) Error() string {
	return fmt.Sprintf("%q missing key in JSON: %q", e.ID, e.JsonKey)
}

// ComponentTranslator is a Translator that converts a JSON config into a component
type ComponentTranslator = Translator[component.Config, component.ID]

// ComponentTranslatorMap is a map-like container which stores ComponentTranslators
type ComponentTranslatorMap = TranslatorMap[component.Config, component.ID]

// ComponentTranslators is a component ID and respective service pipeline.
type ComponentTranslators struct {
	Receivers  ComponentTranslatorMap
	Processors ComponentTranslatorMap
	Exporters  ComponentTranslatorMap
	Extensions ComponentTranslatorMap
}

// PipelineTranslator is a Translator that converts a JSON config into a pipeline
type PipelineTranslator = Translator[*ComponentTranslators, pipeline.ID]

// PipelineTranslatorMap is a map-like container which stores PipelineTranslators
type PipelineTranslatorMap = TranslatorMap[*ComponentTranslators, pipeline.ID]

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

// GetIndexedMap gets the sub map based on the config key and index. If the config value is an array, then the value
// at the index is returned. If it is a map, then the index is ignored and the map is returned directly.
func GetIndexedMap(conf *confmap.Conf, configKey string, index int) map[string]any {
	var got map[string]any
	switch v := conf.Get(configKey).(type) {
	case []any:
		if index != -1 && len(v) > index {
			got = v[index].(map[string]any)
		}
	case map[string]any:
		got = v
	}
	return got
}

// GetMeasurements gets the string values in the measurements section of the provided map. If there are metric
// decoration elements, includes the value associated with the "name" key.
func GetMeasurements(m map[string]any) []string {
	var results []string
	if measurements, ok := m[MeasurementKey].([]any); ok {
		for _, measurement := range measurements {
			switch v := measurement.(type) {
			case string:
				results = append(results, v)
			case map[string]any:
				if n, ok := v[NameKey]; ok {
					if s, ok := n.(string); ok {
						results = append(results, s)
					}
				}
			}
		}
	}
	return results
}

// IsAnySet checks if any of the provided keys are present in the configuration.
func IsAnySet(conf *confmap.Conf, keys []string) bool {
	for _, key := range keys {
		if conf.IsSet(key) {
			return true
		}
	}
	return false
}

func KueueContainerInsightsEnabled(conf *confmap.Conf) bool {
	return GetOrDefaultBool(conf, ConfigKey(LogsKey, MetricsCollectedKey, KubernetesKey, EnableKueueContainerInsights), false)
}

func GetClusterName(conf *confmap.Conf) string {
	val, ok := GetString(conf, ConfigKey(LogsKey, MetricsCollectedKey, KubernetesKey, "cluster_name"))
	if ok && val != "" {
		return val
	}

	envVarClusterName := os.Getenv("K8S_CLUSTER_NAME")
	if envVarClusterName != "" {
		return envVarClusterName
	}

	return util.GetClusterNameFromEc2Tagger()
}
