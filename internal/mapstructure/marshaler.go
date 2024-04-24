// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mapstructure

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
)

func Marshal(rawVal any) (map[string]any, error) {
	enc := New(encoderConfig(rawVal))
	data, err := enc.Encode(rawVal)
	if err != nil {
		return nil, err
	}
	out, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid config encoding")
	}
	return out, nil
}

// encoderConfig returns a default encoder.EncoderConfig that includes an EncodeHook that handles both
// TextMarshaller and confmap.Marshaler interfaces.
func encoderConfig(rawVal any) *EncoderConfig {
	return &EncoderConfig{
		EncodeHook: mapstructure.ComposeDecodeHookFunc(
			NilHookFunc[configopaque.String](),
			NilZeroValueHookFunc[configtls.ServerConfig](),
			TextMarshalerHookFunc(),
			MarshalerHookFunc(rawVal),
			UnsupportedKindHookFunc(),
		),
		NilEmptyMap:   true,
		OmitNilFields: true,
	}
}

// MarshalerHookFunc returns a DecodeHookFuncValue that checks structs that aren't
// the original to see if they implement the Marshaler interface.
func MarshalerHookFunc(orig any) mapstructure.DecodeHookFuncValue {
	origType := reflect.TypeOf(orig)
	return func(from reflect.Value, _ reflect.Value) (any, error) {
		if !from.IsValid() {
			return nil, nil
		}
		if from.Kind() != reflect.Struct {
			return from.Interface(), nil
		}

		// ignore original to avoid infinite loop.
		if from.Type() == origType && reflect.DeepEqual(from.Interface(), orig) {
			return from.Interface(), nil
		}
		marshaler, ok := from.Interface().(confmap.Marshaler)
		if !ok {
			return from.Interface(), nil
		}
		conf := confmap.New()
		if err := marshaler.Marshal(conf); err != nil {
			return nil, err
		}
		return conf.ToStringMap(), nil
	}
}
