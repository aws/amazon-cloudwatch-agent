// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"math"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

func ToOtelValue(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("unsupported value: %v", v)
		}
		return v, nil
	case bool:
		if v {
			return int64(1), nil
		} else {
			return int64(0), nil
		}
	case distribution.Distribution:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
}
