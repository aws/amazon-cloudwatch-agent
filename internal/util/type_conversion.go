// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import "github.com/aws/amazon-cloudwatch-agent/metric/distribution"

func ToOtelValue(value interface{}) interface{} {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	case bool:
		if v {
			return int64(1)
		} else {
			return int64(0)
		}
	case distribution.Distribution:
		return v
	default:
		return nil
	}
}
