package toyamlconfig

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/collector/config"

	encoder "github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	tagNameMapStructure = "mapstructure"
	optionSeparator     = ","
	optionOmitEmpty     = "omitempty"
	optionSquash        = "squash"
	fieldNameSkip       = "-"
)

var (
	componentIDType        = reflect.TypeOf(config.NewComponentID(""))
	errNonStringEncodedKey = errors.New("non string-encoded key")
)

type mapStructureEncoder struct {
}

var _ encoder.Encoder = (*mapStructureEncoder)(nil)

func NewEncoder() encoder.Encoder {
	return &mapStructureEncoder{}
}

func (mse *mapStructureEncoder) Encode(in interface{}, out interface{}) error {
	decoder, err := mapstructure.NewDecoder(mse.Config(out))
	if err != nil {
		return err
	}
	if err = decoder.Decode(in); err != nil {
		return err
	}
	return nil
}

func (mse *mapStructureEncoder) Config(result interface{}) *mapstructure.DecoderConfig {
	return &mapstructure.DecoderConfig{
		Result:           result,
		Metadata:         nil,
		TagName:          tagNameMapStructure,
		WeaklyTypedInput: true,
		DecodeHook:       mse.EncodeHook,
	}
}

func (mse *mapStructureEncoder) EncodeHook(from reflect.Value, _ reflect.Value) (interface{}, error) {
	return mse.encode(from)
}

func (mse *mapStructureEncoder) encode(value reflect.Value) (interface{}, error) {
	if value.IsValid() {
		switch value.Kind() {
		case reflect.Interface, reflect.Ptr:
			return mse.encodeInterfaceOrPtr(value)
		case reflect.Map:
			return mse.encodeMap(value)
		case reflect.Slice:
			return mse.encodeSlice(value)
		case reflect.Struct:
			return mse.encodeStruct(value)
		default:
			return value.Interface(), nil
		}
	}
	return nil, nil
}

func (mse *mapStructureEncoder) encodeInterfaceOrPtr(value reflect.Value) (interface{}, error) {
	if value.Kind() != reflect.Ptr && value.Kind() != reflect.Interface {
		return nil, &reflect.ValueError{
			Method: "encodeInterfaceOrPtr",
			Kind:   value.Kind(),
		}
	}
	return mse.encode(value.Elem())
}

func (mse *mapStructureEncoder) encodeStruct(value reflect.Value) (interface{}, error) {
	if value.Kind() != reflect.Struct {
		return nil, &reflect.ValueError{
			Method: "encodeStruct",
			Kind:   value.Kind(),
		}
	}
	// For structs that need to be handled differently.
	switch value.Type() {
	case componentIDType:
		return value.Interface().(config.ComponentID).String(), nil
	}
	result := make(map[string]interface{})
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.CanInterface() {
			name, omit, squash := mse.getFieldName(value.Type().Field(i))
			if (omit && field.IsZero()) || name == fieldNameSkip {
				continue
			}
			if encoded, err := mse.encode(field); err != nil {
				return nil, err
			} else if squash {
				if m, ok := encoded.(map[string]interface{}); ok {
					for k, v := range m {
						result[k] = v
					}
				}
			} else {
				result[name] = encoded
			}
		}
	}
	return result, nil
}

// getFieldName looks up the mapstructure tag and uses that if available.
// Uses the lowercase field if not found. Checks for omitempty and squash.
func (mse *mapStructureEncoder) getFieldName(field reflect.StructField) (name string, omit bool, squash bool) {
	if tag, ok := field.Tag.Lookup(tagNameMapStructure); ok {
		opts := strings.Split(tag, optionSeparator)
		if len(opts) > 1 {
			for _, opt := range opts {
				if opt == optionOmitEmpty {
					omit = true
				} else if opt == optionSquash {
					squash = true
				}
			}
		}
		return opts[0], omit, squash
	}
	return strings.ToLower(field.Name), false, false
}

func (mse *mapStructureEncoder) encodeSlice(value reflect.Value) (interface{}, error) {
	if value.Kind() != reflect.Slice {
		return nil, &reflect.ValueError{
			Method: "encodeSlice",
			Kind:   value.Kind(),
		}
	}
	result := make([]interface{}, value.Len())
	for i := 0; i < value.Len(); i++ {
		var err error
		if result[i], err = mse.encode(value.Index(i)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (mse *mapStructureEncoder) encodeMap(value reflect.Value) (interface{}, error) {
	if value.Kind() != reflect.Map {
		return nil, &reflect.ValueError{
			Method: "encodeMap",
			Kind:   value.Kind(),
		}
	}
	result := make(map[string]interface{})
	iterator := value.MapRange()
	for iterator.Next() {
		encoded, err := mse.encode(iterator.Key())
		if err != nil {
			return nil, err
		}
		key, ok := encoded.(string)
		if !ok {
			return nil, fmt.Errorf("%w: %v", errNonStringEncodedKey, key)
		}
		if result[key], err = mse.encode(iterator.Value()); err != nil {
			return nil, err
		}
	}
	return result, nil
}
