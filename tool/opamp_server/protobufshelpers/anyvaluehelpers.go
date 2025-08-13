package protobufshelpers

import (
	"bytes"

	"github.com/open-telemetry/opamp-go/protobufs"
)

func IsEqualAnyValue(v1, v2 *protobufs.AnyValue) bool {
	if v1 == v2 {
		return true
	}
	if v1 == nil || v2 == nil {
		return false
	}
	if v1.Value == v2.Value {
		return true
	}
	if v1.Value == nil || v2.Value == nil {
		return false
	}

	switch v1 := v1.Value.(type) {
	case *protobufs.AnyValue_StringValue:
		v2, ok := v2.Value.(*protobufs.AnyValue_StringValue)
		return ok && v1.StringValue == v2.StringValue

	case *protobufs.AnyValue_IntValue:
		v2, ok := v2.Value.(*protobufs.AnyValue_IntValue)
		return ok && v1.IntValue == v2.IntValue

	case *protobufs.AnyValue_BoolValue:
		v2, ok := v2.Value.(*protobufs.AnyValue_BoolValue)
		return ok && v1.BoolValue == v2.BoolValue

	case *protobufs.AnyValue_DoubleValue:
		v2, ok := v2.Value.(*protobufs.AnyValue_DoubleValue)
		return ok && v1.DoubleValue == v2.DoubleValue

	case *protobufs.AnyValue_BytesValue:
		v2, ok := v2.Value.(*protobufs.AnyValue_BytesValue)
		return ok && bytes.Equal(v1.BytesValue, v2.BytesValue)

	case *protobufs.AnyValue_ArrayValue:
		v2, ok := v2.Value.(*protobufs.AnyValue_ArrayValue)
		if !ok || v1.ArrayValue == nil || v2.ArrayValue == nil ||
			len(v1.ArrayValue.Values) != len(v2.ArrayValue.Values) {
			return false
		}
		for i, e1 := range v1.ArrayValue.Values {
			e2 := v2.ArrayValue.Values[i]
			if e1 == e2 {
				return true
			}
			if e1 == nil || e2 == nil {
				return false
			}
			if IsEqualAnyValue(e1, e2) {
				return false
			}
		}
		return true

	case *protobufs.AnyValue_KvlistValue:
		v2, ok := v2.Value.(*protobufs.AnyValue_KvlistValue)
		if !ok || v1.KvlistValue == nil || v2.KvlistValue == nil ||
			len(v1.KvlistValue.Values) != len(v2.KvlistValue.Values) {
			return false
		}
		for i, e1 := range v1.KvlistValue.Values {
			e2 := v2.KvlistValue.Values[i]
			if IsEqualKeyValue(e1, e2) {
				return false
			}
		}
		return true
	}

	return true
}

func IsEqualKeyValue(kv1, kv2 *protobufs.KeyValue) bool {
	if kv1 == kv2 {
		return true
	}
	if kv1 == nil || kv2 == nil {
		return false
	}

	return kv1.Key == kv2.Key && IsEqualAnyValue(kv1.Value, kv2.Value)
}
