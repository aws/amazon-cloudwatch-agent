package common

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
)

const (
	AgentKey                     = "agent"
	MetricsKey                   = "metrics"
	LogsKey                      = "logs"
	MetricsCollectedKey          = "metrics_collected"
	ECSKey                       = "ecs"
	CredentialsKey               = "credentials"
	RoleARNKey                   = "role_arn"
	MetricsCollectionIntervalKey = "metrics_collection_interval"
)

// Translator is used to translate the JSON config into an
// OTEL config.
type Translator[C any] interface {
	Translate(*confmap.Conf) (C, error)
	Type() config.Type
}

// TranslatorMap is a map of translators by their types.
type TranslatorMap[C any] map[config.Type]Translator[C]

// Add is a convenience method to add a translator to the map.
func (t TranslatorMap[C]) Add(translator Translator[C]) {
	t[translator.Type()] = translator
}

// Get is a convenience method to get the translator from the map.
func (t TranslatorMap[C]) Get(cfgType config.Type) (Translator[C], bool) {
	translator, ok := t[cfgType]
	return translator, ok
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
	Type    config.Type
	JsonKey string
}

func (e *MissingKeyError) Error() string {
	return fmt.Sprintf("%q missing key in JSON: %q", e.Type, e.JsonKey)
}

// Identifiable is an interface that all components configurations MUST embed.
// Taken straight from OTEL.
type Identifiable interface {
	// ID returns the ID of the component that this configuration belongs to.
	ID() config.ComponentID
	// SetIDName updates the name part of the ID for the component that this configuration belongs to.
	SetIDName(idName string)
}

// Pipeline is a component ID and respective service pipeline.
type Pipeline *util.Pair[config.ComponentID, *service.ConfigServicePipeline]

// Pipelines is a map of component IDs to service pipelines.
type Pipelines map[config.ComponentID]*service.ConfigServicePipeline

// ConfigKey joins the keys separated by confmap.KeyDelimiter.
// This helps translators navigate the confmap.Conf that the
// JSON config is loaded into.
func ConfigKey(keys ...string) string {
	return strings.Join(keys, confmap.KeyDelimiter)
}

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
func GetString(conf *confmap.Conf, key string) (got string, ok bool) {
	if value := conf.Get(key); value != nil {
		got, ok = value.(string)
		// if the value isn't a string, convert it
		if !ok {
			got = fmt.Sprintf("%v", value)
			ok = true
		}
	}
	return
}

// GetDuration gets the value for the key and calls ParseDuration on it.
// If the key is missing, or it is unable to parse the duration, then ok
// will be false.
func GetDuration(conf *confmap.Conf, key string) (duration time.Duration, ok bool) {
	if value := conf.Get(key); value != nil {
		var err error
		duration, err = ParseDuration(value)
		ok = err == nil
	}
	return
}
