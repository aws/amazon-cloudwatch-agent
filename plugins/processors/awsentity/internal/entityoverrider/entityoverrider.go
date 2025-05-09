package entityoverrider

import (
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// KeyPair represents a key-value pair for entity attributes
type KeyPair struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}

// EntityOverride contains configuration for overriding entity attributes
type EntityOverride struct {
	KeyAttributes []KeyPair `mapstructure:"key_attributes"`
	Attributes    []KeyPair `mapstructure:"attributes"`
}

type EntityOverrider struct {
	overrides *EntityOverride
	logger    *zap.Logger
}

func NewEntityOverrider(overrides *EntityOverride, logger *zap.Logger) *EntityOverrider {
	return &EntityOverrider{
		overrides: overrides,
		logger:    logger,
	}
}

func (p *EntityOverrider) ApplyOverrides(resourceAttrs pcommon.Map) {
	if p.overrides == nil {
		return
	}

	// Apply key attributes
	for _, keyAttr := range p.overrides.KeyAttributes {
		if fullName, ok := entityattributes.GetFullAttributeName(keyAttr.Key); ok {
			resourceAttrs.PutStr(fullName, keyAttr.Value)
		} else {
			p.logger.Debug("Unrecognized key attribute", zap.String("key", keyAttr.Key))
		}
	}

	// Apply additional attributes
	for _, attr := range p.overrides.Attributes {
		if fullName, ok := entityattributes.GetFullAttributeName(attr.Key); ok {
			resourceAttrs.PutStr(fullName, attr.Value)
		} else {
			p.logger.Debug("Unrecognized attribute", zap.String("key", attr.Key))
		}
	}
}

func (p *EntityOverrider) GetOverriddenServiceName() (string, string) {
	if p.overrides == nil {
		return "", ""
	}

	var serviceName, source string
	for _, keyAttr := range p.overrides.KeyAttributes {
		if keyAttr.Key == entityattributes.ServiceName {
			serviceName = keyAttr.Value
		}
	}

	for _, attr := range p.overrides.Attributes {
		if attr.Key == entityattributes.ServiceNameSource {
			source = attr.Value
		}
	}

	return serviceName, source
}
