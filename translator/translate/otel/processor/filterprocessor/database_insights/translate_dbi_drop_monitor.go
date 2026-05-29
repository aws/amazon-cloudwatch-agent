package database_insights

import (
	"fmt"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

type excludeMonitorTranslator struct {
	username string
	index    int
}

func NewExcludeMonitorTranslator(username string, index int) *excludeMonitorTranslator {
	return &excludeMonitorTranslator{username: username, index: index}
}

func (t *excludeMonitorTranslator) ID() component.ID {
	return component.MustNewIDWithName("filter", "dbi_exclude_monitor_"+strconv.Itoa(t.index))
}

func (t *excludeMonitorTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := &filterprocessor.Config{}
	if err := confmap.NewFromStringMap(map[string]interface{}{
		"error_mode": "propagate",
		"logs": map[string]interface{}{
			"log_record": []interface{}{
				fmt.Sprintf(`attributes["user.name"] == "%s" or attributes["postgresql.rolname"] == "%s"`, t.username, t.username),
			},
		},
	}).Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
