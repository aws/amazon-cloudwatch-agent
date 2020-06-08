package util

type MetricArray []interface{}

// We define complete test case to validate from json to toml config
// So sort the metrics components help consistency

func (mArray MetricArray) Len() int {
	return len(mArray)
}

func (mArray MetricArray) Swap(i, j int) {
	mArray[i], mArray[j] = mArray[j], mArray[i]
}

func (mArray MetricArray) Less(i, j int) bool {
	key_i := mArray[i].(map[string]interface{})[Windows_Object_Name_Key].(string)
	key_j := mArray[j].(map[string]interface{})[Windows_Object_Name_Key].(string)
	return key_i < key_j
}
