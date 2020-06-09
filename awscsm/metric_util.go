package awscsmmetrics

func getFloatValue(v interface{}) *float64 {
	ret := float64(0.0)

	switch f := v.(type) {
	case float32:
		ret = float64(f)
	case float64:
		ret = f
	}

	return &ret
}

func getMapFloatValue(v interface{}) map[int64]float64 {
	m := map[int64]float64{}

	tempMap, ok := v.(map[float64]interface{})
	if !ok {
		return m
	}

	for k, v := range tempMap {
		f := getFloatValue(v)
		m[int64(k)] = *f
	}

	return m
}
