// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslogrouterprocessor

import (
	"context"
	"path/filepath"
	"regexp"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type compiledFilter struct {
	isInclude bool
	re        *regexp.Regexp
}

type syslogRouterProcessor struct {
	cfg             *Config
	logger          *zap.Logger
	listenerFilters []compiledFilter
	ruleFilters     []compiledFilter
}

func newProcessor(cfg *Config, logger *zap.Logger) *syslogRouterProcessor {
	p := &syslogRouterProcessor{
		cfg:             cfg,
		logger:          logger,
		listenerFilters: compileFilters(cfg.ListenerFilters),
		ruleFilters:     compileFilters(cfg.RuleFilters),
	}
	if cfg.IsDefault {
		logger.Info("Syslog router initialized for default pipeline",
			zap.Int("all_rules", len(cfg.AllRules)),
			zap.Int("listener_filters", len(cfg.ListenerFilters)),
		)
	} else {
		logger.Info("Syslog router initialized for rule pipeline",
			zap.Any("rule", cfg.Rule),
			zap.Int("prior_rules", len(cfg.PriorRules)),
			zap.Int("listener_filters", len(cfg.ListenerFilters)),
			zap.Int("rule_filters", len(cfg.RuleFilters)),
		)
	}
	return p
}

// compileFilters pre-compiles the regex expressions from the config filters
// into compiledFilter structs for efficient repeated matching.
func compileFilters(filters []Filter) []compiledFilter {
	out := make([]compiledFilter, len(filters))
	for i, f := range filters {
		out[i] = compiledFilter{
			isInclude: f.Type == "include",
			re:        regexp.MustCompile(f.Expression),
		}
	}
	return out
}

// processLogs iterates over all log records in the batch, applies routing and
// filtering via shouldPass, and returns a new plog.Logs containing only the
// records that belong in this pipeline.
func (p *syslogRouterProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	result := plog.NewLogs()
	var total, passed int
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		var hasMatch bool
		var newRL plog.ResourceLogs
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			var scopeMatch bool
			var newSL plog.ScopeLogs
			for k := 0; k < sl.LogRecords().Len(); k++ {
				rec := sl.LogRecords().At(k)
				total++
				if p.shouldPass(rec) {
					passed++
					if !hasMatch {
						newRL = result.ResourceLogs().AppendEmpty()
						rl.Resource().CopyTo(newRL.Resource())
						hasMatch = true
					}
					if !scopeMatch {
						newSL = newRL.ScopeLogs().AppendEmpty()
						sl.Scope().CopyTo(newSL.Scope())
						scopeMatch = true
					}
					rec.CopyTo(newSL.LogRecords().AppendEmpty())
				}
			}
		}
	}
	p.logger.Debug("Batch processed", zap.Int("total", total), zap.Int("passed", passed), zap.Int("dropped", total-passed))
	return result, nil
}

// shouldPass determines whether a log record belongs in this pipeline by applying
// three filtering stages in order: listener-level content filters, attribute-based
// routing (default vs rule pipeline logic), and rule-level content filters.
func (p *syslogRouterProcessor) shouldPass(rec plog.LogRecord) bool {
	body := rec.Body().Str()

	// Step 1: listener-level content filters
	if !passFilters(body, p.listenerFilters) {
		if ce := p.logger.Check(zap.DebugLevel, "Record dropped by listener filter"); ce != nil {
			ce.Write(zap.String("body", truncateBody(body)))
		}
		return false
	}

	// Step 2: attribute-based routing
	if p.cfg.IsDefault {
		// Log diagnostic when a record lacks parsed syslog attributes,
		// indicating the syslog parser failed to parse the message.
		if _, ok := rec.Attributes().Get("hostname"); !ok {
			p.logger.Warn("Syslog message missing parsed attributes (possible parse failure), routing to default pipeline",
				zap.String("body", truncateBody(body)),
			)
		}
		for _, r := range p.cfg.AllRules {
			if matchRule(r, rec) {
				if ce := p.logger.Check(zap.DebugLevel, "Record claimed by explicit rule, excluding from default pipeline"); ce != nil {
					ce.Write(zap.Any("matched_rule", r))
				}
				return false
			}
		}
	} else {
		if !matchRule(p.cfg.Rule, rec) {
			return false
		}
		for _, pr := range p.cfg.PriorRules {
			if matchRule(pr, rec) {
				if ce := p.logger.Check(zap.DebugLevel, "Record preempted by higher-priority rule"); ce != nil {
					ce.Write(zap.Any("prior_rule", pr))
				}
				return false
			}
		}
	}

	// Step 3: rule-level content filters
	if !passFilters(body, p.ruleFilters) {
		if ce := p.logger.Check(zap.DebugLevel, "Record dropped by rule filter"); ce != nil {
			ce.Write(zap.String("body", truncateBody(body)))
		}
		return false
	}

	return true
}

// truncateBody limits the log body to 200 characters in diagnostic log messages
// to prevent large syslog payloads from flooding the agent's own log file.
func truncateBody(body string) string {
	if len(body) > 200 {
		return body[:200] + "..."
	}
	return body
}

// passFilters returns true if the log body passes all exclude filters and matches
// at least one include filter (when include filters are present).
func passFilters(body string, filters []compiledFilter) bool {
	hasInclude := false
	includeMatched := false
	for _, f := range filters {
		if !f.isInclude {
			if f.re.MatchString(body) {
				return false
			}
		} else {
			hasInclude = true
			if !includeMatched && f.re.MatchString(body) {
				includeMatched = true
			}
		}
	}
	if hasInclude && !includeMatched {
		return false
	}
	return true
}

// matchRule returns true if the log record's attributes satisfy all non-empty
// fields in the rule (hostname, facility, appname) using glob matching for strings.
func matchRule(r MatchRule, rec plog.LogRecord) bool {
	attrs := rec.Attributes()
	if r.Hostname != "" {
		v, ok := attrs.Get("hostname")
		if !ok {
			return false
		}
		if matched, _ := filepath.Match(r.Hostname, v.Str()); !matched {
			return false
		}
	}
	if r.Facility != nil {
		v, ok := attrs.Get("facility")
		if !ok {
			return false
		}
		if v.Int() != int64(*r.Facility) {
			return false
		}
	}
	if r.AppName != "" {
		v, ok := attrs.Get("appname")
		if !ok {
			return false
		}
		if matched, _ := filepath.Match(r.AppName, v.Str()); !matched {
			return false
		}
	}
	return true
}
