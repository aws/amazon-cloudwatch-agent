// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mapstructure

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtelemetry"
)

type inter interface {
	run()
}

type complexStruct struct {
	Skipped   emptyStruct             `mapstructure:",squash"`
	Nested    simpleStruct            `mapstructure:",squash"`
	Slice     []simpleStruct          `mapstructure:"slice,omitempty"`
	Pointer   *simpleStruct           `mapstructure:"ptr"`
	Map       map[string]simpleStruct `mapstructure:"map,omitempty"`
	Interface inter
}

type simpleStruct struct {
	Value   string `mapstructure:"value"`
	skipped string
}

func (*simpleStruct) run() {
}

type emptyStruct struct {
	Value string `mapstructure:"-"`
}

func TestEncode(t *testing.T) {
	encoder := &mapStructureEncoder{}
	testCases := map[string]struct {
		value      func() reflect.Value
		wantResult interface{}
	}{
		"WithString": {
			value: func() reflect.Value {
				return reflect.ValueOf("test")
			},
			wantResult: "test",
		},
		"WithNil": {
			value: func() reflect.Value {
				return reflect.ValueOf(nil)
			},
			wantResult: nil,
		},
		"WithConfigTelemetry": {
			value: func() reflect.Value {
				return reflect.ValueOf(configtelemetry.LevelNone)
			},
			wantResult: "none",
		},
		"WithComponentID": {
			value: func() reflect.Value {
				return reflect.ValueOf(config.NewComponentIDWithName("type", "name"))
			},
			wantResult: "type/name",
		},
		"WithSlice": {
			value: func() reflect.Value {
				s := []config.ComponentID{
					config.NewComponentID("nop"),
					config.NewComponentIDWithName("type", "name"),
				}
				return reflect.ValueOf(s)
			},
			wantResult: []interface{}{"nop", "type/name"},
		},
		"WithSimpleStruct": {
			value: func() reflect.Value {
				return reflect.ValueOf(simpleStruct{Value: "test", skipped: "skipped"})
			},
			wantResult: map[string]interface{}{
				"value": "test",
			},
		},
		"WithComplexStruct": {
			value: func() reflect.Value {
				c := complexStruct{
					Skipped: emptyStruct{
						Value: "omitted",
					},
					Nested: simpleStruct{
						Value: "nested",
					},
					Slice: []simpleStruct{
						{Value: "slice"},
					},
					Map: map[string]simpleStruct{
						"Key": {Value: "map"},
					},
					Pointer: &simpleStruct{
						Value: "pointer",
					},
					Interface: &simpleStruct{Value: "interface"},
				}
				return reflect.ValueOf(&c)
			},
			wantResult: map[string]interface{}{
				"value": "nested",
				"slice": []interface{}{map[string]interface{}{"value": "slice"}},
				"map": map[string]interface{}{
					"Key": map[string]interface{}{"value": "map"},
				},
				"ptr":       map[string]interface{}{"value": "pointer"},
				"interface": map[string]interface{}{"value": "interface"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := encoder.encode(testCase.value())
			require.NoError(t, err)
			require.Equal(t, testCase.wantResult, got)
		})
	}
}

func TestGetFieldName(t *testing.T) {
	encoder := &mapStructureEncoder{}
	testCases := map[string]struct {
		field      reflect.StructField
		wantName   string
		wantOmit   bool
		wantSquash bool
	}{
		"WithoutTags": {
			field: reflect.StructField{
				Name: "Test",
			},
			wantName: "test",
		},
		"WithoutMapStructureTag": {
			field: reflect.StructField{
				Tag:  `yaml:"hello,inline"`,
				Name: "YAML",
			},
			wantName: "yaml",
		},
		"WithRename": {
			field: reflect.StructField{
				Tag:  `mapstructure:"hello"`,
				Name: "Test",
			},
			wantName: "hello",
		},
		"WithOmitEmpty": {
			field: reflect.StructField{
				Tag:  `mapstructure:"hello,omitempty"`,
				Name: "Test",
			},
			wantName: "hello",
			wantOmit: true,
		},
		"WithSquash": {
			field: reflect.StructField{
				Tag:  `mapstructure:",squash"`,
				Name: "Test",
			},
			wantSquash: true,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			gotName, gotOmit, gotSquash := encoder.getFieldName(testCase.field)
			require.Equal(t, testCase.wantName, gotName)
			require.Equal(t, testCase.wantOmit, gotOmit)
			require.Equal(t, testCase.wantSquash, gotSquash)
		})
	}
}

func TestEncodeValueError(t *testing.T) {
	encoder := &mapStructureEncoder{}
	testValue := reflect.ValueOf("")
	testEncodes := []func(value reflect.Value) (interface{}, error){
		encoder.encodeMap,
		encoder.encodeStruct,
		encoder.encodeSlice,
	}
	for _, testEncode := range testEncodes {
		got, err := testEncode(testValue)
		require.Error(t, err)
		require.Nil(t, got)
	}
}

func TestEncodeNonStringEncodedKey(t *testing.T) {
	testCase := map[simpleStruct]simpleStruct{
		{Value: "key"}: {Value: "value"},
	}
	encoder := &mapStructureEncoder{}
	got, err := encoder.encodeMap(reflect.ValueOf(testCase))
	require.Error(t, err)
	require.Nil(t, got)
}

func TestEncoder(t *testing.T) {
	encoder := NewEncoder()
	t.Run("WithValid", func(t *testing.T) {
		var got map[string]interface{}
		err := encoder.Encode(simpleStruct{Value: "test"}, &got)
		require.NoError(t, err)
		require.Equal(t, map[string]interface{}{"value": "test"}, got)
	})
	t.Run("WithInvalidResultType", func(t *testing.T) {
		var got map[string]simpleStruct
		err := encoder.Encode(simpleStruct{Value: "test"}, &got)
		require.Error(t, err)
	})
	t.Run("WithNonPointerResult", func(t *testing.T) {
		var got interface{}
		err := encoder.Encode(simpleStruct{Value: "test"}, got)
		require.Error(t, err)
	})
}
