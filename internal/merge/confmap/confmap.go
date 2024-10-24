// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package confmap

import (
	"github.com/knadh/koanf/maps"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	otelconfmap "go.opentelemetry.io/collector/confmap"
)

const (
	KeyDelimiter = otelconfmap.KeyDelimiter
)

type Conf struct {
	k *koanf.Koanf
}

func New() *Conf {
	return &Conf{k: koanf.New(KeyDelimiter)}
}

func NewFromStringMap(data map[string]any) *Conf {
	m := New()
	// Cannot return error because the koanf instance is empty.
	_ = m.k.Load(confmap.Provider(data, ""), nil)
	return m
}

func (c *Conf) Merge(in *Conf) error {
	if in == nil {
		return nil
	}
	return c.mergeFromStringMap(in.ToStringMap())
}

func (c *Conf) mergeFromStringMap(data map[string]any) error {
	return c.k.Load(confmap.Provider(data, ""), nil, koanf.WithMergeFunc(mergeMaps))
}

func (c *Conf) ToStringMap() map[string]any {
	return maps.Unflatten(c.k.All(), KeyDelimiter)
}
