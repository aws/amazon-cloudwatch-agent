package util

const (
	Data_Format_Key     = "data_format"
	Data_Format_Value   = "value"
	Data_Type_Key       = "data_type"
	Data_Type_Value     = "string"
	Name_Override_Key   = "name_override"
	Name_Override_Value = "raw_log_line"
)

func AddFixedTailConfig(input map[string]interface{}) {
	input[Data_Format_Key] = Data_Format_Value
	input[Data_Type_Key] = Data_Type_Value
	input[Name_Override_Key] = Name_Override_Value
}
