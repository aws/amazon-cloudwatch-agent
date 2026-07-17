// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filelog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/timestamp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
)

type Option func(*translator)

type translator struct {
	factory          receiver.Factory
	name             string
	filePath         string
	namePrefix       string
	index            int
	encoding         string
	multilinePattern string
	timestampFormat  string
	timezone         string
	severity         string
	resource         map[string]string
	useStorage       bool
	startAtBeginning bool
}

func (t *translator) startAt() string {
	if t.startAtBeginning {
		return "beginning"
	}
	return "end"
}

var _ common.ComponentTranslator = (*translator)(nil)

func WithFilePath(filePath string) Option {
	return func(t *translator) { t.filePath = filePath }
}

func WithIndex(index int) Option {
	return func(t *translator) { t.index = index }
}

func WithNamePrefix(prefix string) Option {
	return func(t *translator) { t.namePrefix = prefix }
}

func WithName(name string) Option {
	return func(t *translator) { t.name = name }
}

func WithEncoding(encoding string) Option {
	return func(t *translator) { t.encoding = encoding }
}

func WithMultilinePattern(pattern string) Option {
	return func(t *translator) { t.multilinePattern = pattern }
}

func WithTimestampFormat(format, timezone string) Option {
	return func(t *translator) {
		t.timestampFormat = format
		t.timezone = timezone
	}
}

func WithSeverityPattern(pattern string) Option {
	return func(t *translator) { t.severity = pattern }
}

func WithResource(resource map[string]string) Option {
	return func(t *translator) { t.resource = resource }
}

func WithStartAtBeginning() Option {
	return func(t *translator) { t.startAtBeginning = true }
}

func WithStorage() Option {
	return func(t *translator) { t.useStorage = true }
}

func NewTranslator(opts ...Option) common.ComponentTranslator {
	t := &translator{factory: filelogreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	name := t.name
	if name == "" {
		name = t.namePrefix + "_" + strconv.Itoa(t.index)
	}
	return component.MustNewIDWithName("filelog", name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	// Timestamp/severity parsing requires regex_parser operators. operator.Config has no Marshal
	// method, so confmap serializes it as a raw Go struct (wrapped in "builder:", with
	// field types like parse_from rendered as {fieldinterface: {keys: []}}) instead of
	// the string format the receiver expects. Use a raw map to bypass this.
	if t.timestampFormat != "" {
		return t.translateAsRawMap()
	}
	return t.translateTyped()
}

func (t *translator) translateTyped() (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filelogreceiver.FileLogConfig)
	cfg.InputConfig.Include = []string{t.filePath}
	cfg.InputConfig.StartAt = t.startAt()

	encoding := t.encoding
	if encoding == "" {
		encoding = "utf-8"
	}
	cfg.InputConfig.Encoding = encoding

	if t.multilinePattern != "" {
		cfg.InputConfig.SplitConfig.LineStartPattern = t.multilinePattern
	}

	if t.useStorage {
		storageID := filestorage.ComponentID()
		cfg.StorageID = &storageID
	}

	if len(t.resource) > 0 {
		cfg.InputConfig.Resource = make(map[string]helper.ExprStringConfig, len(t.resource))
		for k, v := range t.resource {
			cfg.InputConfig.Resource[k] = helper.ExprStringConfig(v)
		}
	}

	return cfg, nil
}

// translateAsRawMap builds the receiver config as a raw map to work around the
// operator.Config marshaling bug that breaks confmap round-tripping when operators are present.
func (t *translator) translateAsRawMap() (component.Config, error) {
	timestampRegex := timestamp.BuildRegexWithNamedCaptureGroup(t.timestampFormat)
	if _, err := regexp.Compile(timestampRegex); err != nil {
		return nil, fmt.Errorf("timestamp_format %q produces invalid regex for %s: %w", t.timestampFormat, t.filePath, err)
	}

	encoding := t.encoding
	if encoding == "" {
		encoding = "utf-8"
	}

	cfgMap := map[string]any{
		"include":  []string{t.filePath},
		"start_at": t.startAt(),
		"encoding": encoding,
	}

	if t.multilinePattern != "" {
		cfgMap["multiline"] = map[string]any{
			"line_start_pattern": t.multilinePattern,
		}
	}

	if t.useStorage {
		cfgMap["storage"] = filestorage.ComponentID().String()
	}

	if len(t.resource) > 0 {
		cfgMap["resource"] = t.resource
	}

	var operators []any
	if t.timestampFormat != "" {
		operators = append(operators, buildTimestampOperatorMap(t.timestampFormat, t.timezone))
	}
	if t.severity != "" {
		operators = append(operators, buildSeverityOperatorMap(t.severity))
	}
	if len(operators) > 0 {
		cfgMap["operators"] = operators
	}

	return &rawMapConfig{data: cfgMap}, nil
}

// buildTimestampOperatorMap returns a regex_parser operator config as a raw map.
// Using a raw map avoids the operator.Config marshaling bug when round-tripping through confmap.
func buildTimestampOperatorMap(format, timezone string) map[string]any {
	location := "Local"
	if strings.EqualFold(timezone, "UTC") {
		location = "UTC"
	}

	return map[string]any{
		"type":  "regex_parser",
		"regex": timestamp.BuildRegexWithNamedCaptureGroup(format),
		"timestamp": map[string]any{
			"parse_from":  "attributes.timestamp",
			"layout":      timestamp.BuildLayout(format),
			"layout_type": "gotime",
			"location":    location,
		},
	}
}

func buildSeverityOperatorMap(severity string) map[string]any {
	return map[string]any{
		"type":  "regex_parser",
		"regex": severity,
		"severity": map[string]any{
			"parse_from": "attributes.severity",
			"mapping": map[string]any{
				"debug": []string{"DEBUG", "DEBUG1", "DEBUG2", "DEBUG3", "DEBUG4", "DEBUG5"},
				"info":  []string{"LOG", "INFO", "NOTICE", "STATEMENT"},
				"warn":  "WARNING",
				"error": "ERROR",
				"fatal": []string{"FATAL", "PANIC"},
			},
		},
	}
}

// rawMapConfig wraps a raw config map and passes it through serialization unchanged.
// This avoids the operator.Config marshaling bug when round-tripping through confmap.
type rawMapConfig struct {
	data map[string]any
}

var _ component.Config = (*rawMapConfig)(nil)
var _ confmap.Marshaler = rawMapConfig{}

func (r rawMapConfig) Validate() error { return nil }

func (r rawMapConfig) Marshal(conf *confmap.Conf) error {
	return conf.Merge(confmap.NewFromStringMap(r.data))
}
