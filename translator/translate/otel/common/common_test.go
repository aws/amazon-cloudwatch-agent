// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

type testTranslator struct {
	cfgType component.Type
	result  int
}

var _ Translator[int] = (*testTranslator)(nil)

func (t testTranslator) Translate(_ *confmap.Conf) (int, error) {
	return t.result, nil
}

func (t testTranslator) ID() component.ID {
	return component.NewID(t.cfgType)
}

func TestConfigKeys(t *testing.T) {
	require.Equal(t, "1::2", ConfigKey("1", "2"))
}

func TestGetString(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{"int": 10, "string": "test"})
	got, ok := GetString(conf, "int")
	require.True(t, ok)
	// converts int to string
	require.Equal(t, "10", got)
	got, ok = GetString(conf, "string")
	require.True(t, ok)
	require.Equal(t, "test", got)
	got, ok = GetString(conf, "invalid_key")
	require.False(t, ok)
	require.Equal(t, "", got)
}

func TestGetBool(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{"int": 10, "string": "test", "bool1": false, "bool2": true})
	got, ok := GetBool(conf, "int")
	require.False(t, ok)
	require.False(t, got)

	got, ok = GetBool(conf, "string")
	require.False(t, ok)
	require.False(t, got)

	got, ok = GetBool(conf, "bool1")
	require.True(t, ok)
	require.False(t, got)

	got, ok = GetBool(conf, "bool2")
	require.True(t, ok)
	require.True(t, got)
}

func TestGetDuration(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{"invalid": "invalid", "valid": 1, "zero": 0})
	got, ok := GetDuration(conf, "invalid")
	require.False(t, ok)
	require.Equal(t, time.Duration(0), got)
	got, ok = GetDuration(conf, "valid")
	require.True(t, ok)
	require.Equal(t, time.Second, got)
	got, ok = GetDuration(conf, "zero")
	require.False(t, ok)
	require.Equal(t, time.Duration(0), got)
}

func TestParseDuration(t *testing.T) {
	testCases := map[string]struct {
		input   interface{}
		want    time.Duration
		wantErr bool
	}{
		"WithString/Duration": {input: "60s", want: time.Minute},
		"WithString/Int":      {input: "60", want: time.Minute},
		"WithString/Float":    {input: "60.7", want: time.Minute},
		"WithString/Invalid":  {input: "test", wantErr: true},
		"WithInt":             {input: 60, want: time.Minute},
		"WithInt64":           {input: int64(60), want: time.Minute},
		"WithFloat64":         {input: 59.7, want: 59 * time.Second},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseDuration(testCase.input)
			if testCase.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestTranslatorMap(t *testing.T) {
	got := NewTranslatorMap[int](&testTranslator{"test", 0}, &testTranslator{"other", 1})
	require.Len(t, got, 2)
	translator, ok := got.Get(component.NewID("test"))
	require.True(t, ok)
	result, err := translator.Translate(nil)
	require.NoError(t, err)
	require.Equal(t, 0, result)
	other := NewTranslatorMap[int](&testTranslator{"test", 2})
	got.Merge(other)
	require.Len(t, got, 2)
	translator, ok = got.Get(component.NewID("test"))
	require.True(t, ok)
	result, err = translator.Translate(nil)
	require.NoError(t, err)
	require.Equal(t, 2, result)
	require.Equal(t, []component.ID{component.NewID("other"), component.NewID("test")}, got.SortedKeys())
}

func TestMissingKeyError(t *testing.T) {
	err := &MissingKeyError{ID: component.NewID("type"), JsonKey: "key"}
	require.Equal(t, "\"type\" missing key in JSON: \"key\"", err.Error())
}

func TestGetOrDefaultDuration(t *testing.T) {
	sectionKeys := []string{"section::metrics_collection_interval", "backup::metrics_collection_interval"}
	testCases := map[string]struct {
		input map[string]interface{}
		want  time.Duration
	}{
		"WithDefault": {
			input: map[string]interface{}{},
			want:  time.Minute,
		},
		"WithZeroInterval": {
			input: map[string]interface{}{
				"backup": map[string]interface{}{
					"metrics_collection_interval": 0,
				},
				"section": map[string]interface{}{
					"metrics_collection_interval": 0,
				},
			},
			want: time.Minute,
		},
		"WithoutSectionOverride": {
			input: map[string]interface{}{
				"backup": map[string]interface{}{
					"metrics_collection_interval": 10,
				},
				"section": map[string]interface{}{},
			},
			want: 10 * time.Second,
		},
		"WithInvalidSectionOverride": {
			input: map[string]interface{}{
				"backup": map[string]interface{}{
					"metrics_collection_interval": 10,
				},
				"section": map[string]interface{}{
					"metrics_collection_interval": "invalid",
				},
			},
			want: 10 * time.Second,
		},
		"WithSectionOverride": {
			input: map[string]interface{}{
				"backup": map[string]interface{}{
					"metrics_collection_interval": 10,
				},
				"section": map[string]interface{}{
					"metrics_collection_interval": 120,
				},
			},
			want: 2 * time.Minute,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got := GetOrDefaultDuration(conf, sectionKeys, time.Minute)
			require.Equal(t, testCase.want, got)
		})
	}
}
