// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package otlp wraps the upstream OTLP receiver so that profiles whose resource and
// scope attributes arrive as ProfilesDictionary string-table references are resolved
// before they reach the pipeline. pdata v1.56 only resolves those references in
// pprofile.ProtoUnmarshaler, which the receiver does not use, so processors saw empty
// attribute keys. Upstream fixed this in opentelemetry-collector v0.161.0
// (open-telemetry/opentelemetry-collector#15794); remove this package once the agent
// is on that version or later.
package otlp

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.uber.org/zap"
)

func NewFactory() receiver.Factory {
	base, ok := otlpreceiver.NewFactory().(xreceiver.Factory)
	if !ok {
		panic("receiver/otlp: upstream otlpreceiver.NewFactory() no longer implements xreceiver.Factory")
	}
	return xreceiver.NewFactory(
		base.Type(),
		base.CreateDefaultConfig,
		xreceiver.WithTraces(base.CreateTraces, base.TracesStability()),
		xreceiver.WithMetrics(base.CreateMetrics, base.MetricsStability()),
		xreceiver.WithLogs(base.CreateLogs, base.LogsStability()),
		xreceiver.WithProfiles(createProfiles(base), base.ProfilesStability()),
	)
}

func createProfiles(base xreceiver.Factory) xreceiver.CreateProfilesFunc {
	return func(ctx context.Context, set receiver.Settings, cfg component.Config, next xconsumer.Profiles) (xreceiver.Profiles, error) {
		resolving, err := xconsumer.NewProfiles(func(ctx context.Context, pd pprofile.Profiles) error {
			return next.ConsumeProfiles(ctx, resolveReferences(set.Logger, pd))
		}, consumer.WithCapabilities(consumer.Capabilities{MutatesData: true}))
		if err != nil {
			return nil, err
		}
		return base.CreateProfiles(ctx, set, cfg, resolving)
	}
}

// resolveReferences round-trips the profiles through the pprofile proto marshalers,
// whose unmarshaler is the only place pdata v1.56 resolves attribute references.
// The marshaler converts the input in place, so the round-trip runs on a copy and the
// original is forwarded untouched if either step fails.
func resolveReferences(logger *zap.Logger, pd pprofile.Profiles) pprofile.Profiles {
	if !hasUnresolvedReferences(pd) {
		return pd
	}
	working := pprofile.NewProfiles()
	pd.CopyTo(working)
	buf, err := (&pprofile.ProtoMarshaler{}).MarshalProfiles(working)
	if err != nil {
		logger.Warn("Failed to resolve profile attribute references, forwarding unresolved", zap.Error(err))
		return pd
	}
	resolved, err := (&pprofile.ProtoUnmarshaler{}).UnmarshalProfiles(buf)
	if err != nil {
		logger.Warn("Failed to resolve profile attribute references, forwarding unresolved", zap.Error(err))
		return pd
	}
	for _, rp := range resolved.ResourceProfiles().All() {
		inlineKeys(rp.Resource().Attributes())
		for _, sp := range rp.ScopeProfiles().All() {
			inlineKeys(sp.Scope().Attributes())
		}
	}
	return resolved
}

// inlineKeys rebuilds the map so the exported attributes carry only key, not key and
// key_strindex together, which the OTLP KeyValue definition forbids.
func inlineKeys(attributes pcommon.Map) {
	resolved := pcommon.NewMap()
	attributes.CopyTo(resolved)
	attributes.Clear()
	for key, value := range resolved.All() {
		value.CopyTo(attributes.PutEmpty(key))
	}
}

func hasUnresolvedReferences(pd pprofile.Profiles) bool {
	if pd.Dictionary().StringTable().Len() == 0 {
		return false
	}
	for _, rp := range pd.ResourceProfiles().All() {
		if hasUnresolvedAttribute(rp.Resource().Attributes()) {
			return true
		}
		for _, sp := range rp.ScopeProfiles().All() {
			if hasUnresolvedAttribute(sp.Scope().Attributes()) {
				return true
			}
		}
	}
	return false
}

func hasUnresolvedAttribute(attributes pcommon.Map) bool {
	for key, value := range attributes.All() {
		if key == "" || value.Type() == pcommon.ValueTypeEmpty {
			return true
		}
	}
	return false
}
