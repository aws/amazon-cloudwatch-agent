// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"golang.org/x/exp/maps"
)

const (
	AgentKey                     = "agent"
	MetricsKey                   = "metrics"
	LogsKey                      = "logs"
	TracesKey                    = "traces"
	MetricsCollectedKey          = "metrics_collected"
	LogsCollectedKey             = "logs_collected"
	TracesCollectedKey           = "traces_collected"
	ECSKey                       = "ecs"
	KubernetesKey                = "kubernetes"
	PrometheusKey                = "prometheus"
	EMFProcessorKey              = "emf_processor"
	XrayKey                      = "xray"
	EndpointOverrideKey          = "endpoint_override"
	RegionOverrideKey            = "region_override"
	ProxyOverrideKey             = "proxy_override"
	InsecureKey                  = "insecure"
	LocalModeKey                 = "local_mode"
	CredentialsKey               = "credentials"
	RoleARNKey                   = "role_arn"
	MetricsCollectionIntervalKey = "metrics_collection_interval"
	Console                      = "console"
	DiskIOKey                    = "diskio"
	NetKey                       = "net"
	Emf                          = "emf"
	ServiceAddress               = "service_address"
	Udp                          = "udp"
	Tcp                          = "tcp"
	Region                       = "region"
	LogGroupName                 = "log_group_name"
	LogStreamName                = "log_stream_name"
)

const (
	PipelineNameHost             = "host"
	PipelineNameHostDeltaMetrics = "hostDeltaMetrics"
	PipelineNameEmfLogs          = "emf_logs"
)

// Translator is used to translate the JSON config into an
// OTEL config.
type Translator[C any] interface {
	Translate(*confmap.Conf) (C, error)
	ID() component.ID
}

// TranslatorMap is a map of translators by their types.
type TranslatorMap[C any] map[component.ID]Translator[C]

// Add is a convenience method to add a translator to the map.
func (t TranslatorMap[C]) Add(translator Translator[C]) {
	t[translator.ID()] = translator
}

// Get is a convenience method to get the translator from the map.
func (t TranslatorMap[C]) Get(id component.ID) (Translator[C], bool) {
	translator, ok := t[id]
	return translator, ok
}

// Merge adds the translators in the input to the existing map.
func (t TranslatorMap[C]) Merge(m TranslatorMap[C]) {
	for _, v := range m {
		t.Add(v)
	}
}

// SortedKeys returns the sorted component.ID keys.
func (t TranslatorMap[C]) SortedKeys() []component.ID {
	keys := maps.Keys(t)
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})
	return keys
}

// NewTranslatorMap creates a TranslatorMap from the translators.
func NewTranslatorMap[C any](translators ...Translator[C]) TranslatorMap[C] {
	translatorMap := make(TranslatorMap[C], len(translators))
	for _, translator := range translators {
		translatorMap.Add(translator)
	}
	return translatorMap
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
		got, _ := value.([]C)
		return got
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

// GetNumber gets the number value for the key. If the key is missing or
// the value is not a float64 type (default JSON unmarshal maps number to
// float64), then ok will be false.
func GetNumber(conf *confmap.Conf, key string) (value float64, ok bool) {
	if v := conf.Get(key); v != nil {
		value, ok = v.(float64)
	}
	return
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
