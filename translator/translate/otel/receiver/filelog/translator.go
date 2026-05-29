package filelog

import (
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type Option func(*translator)

type translator struct {
	factory    receiver.Factory
	filePath   string
	namePrefix string
	index      int
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

func NewTranslator(opts ...Option) common.ComponentTranslator {
	t := &translator{factory: filelogreceiver.NewFactory(), namePrefix: "postgresql"}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.MustNewIDWithName("filelog", t.namePrefix+"_"+strconv.Itoa(t.index))
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filelogreceiver.FileLogConfig)
	cfg.InputConfig.Include = []string{t.filePath}
	cfg.InputConfig.StartAt = "end"
	cfg.InputConfig.Encoding = "utf-8"
	return cfg, nil
}
