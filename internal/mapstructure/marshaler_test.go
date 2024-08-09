// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mapstructure

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

type TestConfig struct {
	Boolean   *bool              `mapstructure:"boolean"`
	Struct    *Struct            `mapstructure:"struct"`
	MapStruct map[string]*Struct `mapstructure:"map_struct"`
}

func (t TestConfig) Marshal(conf *confmap.Conf) error {
	if t.Boolean != nil && !*t.Boolean {
		return errors.New("unable to marshal")
	}
	if err := conf.Marshal(t); err != nil {
		return err
	}
	return conf.Merge(confmap.NewFromStringMap(map[string]any{
		"additional": "field",
	}))
}

type Struct struct {
	Name string
}

type TestIDConfig struct {
	Boolean bool              `mapstructure:"bool"`
	Map     map[TestID]string `mapstructure:"map"`
}

func TestMarshal(t *testing.T) {
	cfg := &TestIDConfig{
		Boolean: true,
		Map: map[TestID]string{
			"string": "this is a string",
		},
	}
	got, err := Marshal(cfg)
	assert.NoError(t, err)
	conf := confmap.NewFromStringMap(got)
	assert.Equal(t, true, conf.Get("bool"))
	assert.Equal(t, map[string]any{"string_": "this is a string"}, conf.Get("map"))
}

func TestMarshalDuplicateID(t *testing.T) {
	cfg := &TestIDConfig{
		Boolean: true,
		Map: map[TestID]string{
			"string":  "this is a string",
			"string_": "this is another string",
		},
	}
	_, err := Marshal(cfg)
	assert.Error(t, err)
}

func TestMarshalError(t *testing.T) {
	_, err := Marshal(nil)
	assert.Error(t, err)
}

func TestMarshaler(t *testing.T) {
	cfg := &TestConfig{
		Struct: &Struct{
			Name: "StructName",
		},
	}
	got, err := Marshal(cfg)
	assert.NoError(t, err)
	conf := confmap.NewFromStringMap(got)
	assert.Equal(t, "field", conf.Get("additional"))

	type NestedMarshaler struct {
		TestConfig *TestConfig
	}
	nmCfg := &NestedMarshaler{
		TestConfig: cfg,
	}
	got, err = Marshal(nmCfg)
	assert.NoError(t, err)
	conf = confmap.NewFromStringMap(got)
	sub, err := conf.Sub("testconfig")
	assert.NoError(t, err)
	assert.True(t, sub.IsSet("additional"))
	assert.Equal(t, "field", sub.Get("additional"))
	varBool := false
	nmCfg.TestConfig.Boolean = &varBool
	assert.Error(t, conf.Marshal(nmCfg))
}
