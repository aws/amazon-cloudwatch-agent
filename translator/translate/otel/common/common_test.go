// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestGetArray(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"int":    []any{5, 8, 10},
		"string": []any{"bool", "empty"},
	})
	gotInt := GetArray[int](conf, "int")
	require.Equal(t, []int{5, 8, 10}, gotInt)

	gotInt = GetArray[int](conf, "int-val")
	require.Equal(t, []int(nil), gotInt)

	gotStr := GetArray[string](conf, "int")
	require.Equal(t, []string(nil), gotStr)

	gotStr = GetArray[string](conf, "string")
	require.Equal(t, []string{"bool", "empty"}, gotStr)

	gotStr = GetArray[string](conf, "string-val")
	require.Equal(t, []string(nil), gotStr)
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

func TestGetOrDefaultBool(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{"int": 10, "string": "test", "bool1": false, "bool2": true})
	got := GetOrDefaultBool(conf, "int", false)
	require.False(t, got)

	got = GetOrDefaultBool(conf, "string", true)
	require.True(t, got)

	got = GetOrDefaultBool(conf, "bool1", true)
	require.False(t, got)

	got = GetOrDefaultBool(conf, "bool2", true)
	require.True(t, got)

	got = GetOrDefaultBool(conf, "non_existent_key", true)
	require.True(t, got)
}

func TestGetNumber(t *testing.T) {
	test := map[string]interface{}{"int": 10, "string": "test", "bool": false, "float": 1.3}
	marshalled, err := json.Marshal(test)
	require.NoError(t, err)
	var unmarshalled map[string]interface{}
	require.NoError(t, json.Unmarshal(marshalled, &unmarshalled))

	conf := confmap.NewFromStringMap(unmarshalled)
	got, ok := GetNumber(conf, "int")
	assert.True(t, ok)
	assert.Equal(t, 10.0, got)

	got, ok = GetNumber(conf, "string")
	assert.False(t, ok)
	assert.Equal(t, 0.0, got)

	got, ok = GetNumber(conf, "bool")
	assert.False(t, ok)
	assert.Equal(t, 0.0, got)

	got, ok = GetNumber(conf, "float")
	assert.True(t, ok)
	assert.Equal(t, 1.3, got)
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
	firstType, _ := component.NewType("first")
	middleType, _ := component.NewType("middle")
	lastType, _ := component.NewType("last")
	got := NewTranslatorMap[int](&testTranslator{firstType, 0}, &testTranslator{middleType, 1})
	require.Equal(t, 2, got.Len())
	translator, ok := got.Get(component.NewID(firstType))
	require.True(t, ok)
	result, err := translator.Translate(nil)
	require.NoError(t, err)
	require.Equal(t, 0, result)
	other := NewTranslatorMap[int](&testTranslator{firstType, 2}, &testTranslator{lastType, 3})
	got.Merge(other)
	require.Equal(t, 3, got.Len())
	translator, ok = got.Get(component.NewID(firstType))
	require.True(t, ok)
	result, err = translator.Translate(nil)
	require.NoError(t, err)
	require.Equal(t, 2, result)
	require.Equal(t, []component.ID{component.NewID(firstType), component.NewID(middleType), component.NewID(lastType)}, got.Keys())
}

func TestMissingKeyError(t *testing.T) {
	newType, _ := component.NewType("type")
	err := &MissingKeyError{ID: component.NewID(newType), JsonKey: "key"}
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
