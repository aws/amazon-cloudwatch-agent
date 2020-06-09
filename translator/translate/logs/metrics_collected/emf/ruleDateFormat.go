package emf

type DataFormat struct {
}

const SectionKeyDataFormat = "data_format"

func (obj *DataFormat) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	return SectionKeyDataFormat, "emf"
}

func init() {
	obj := new(DataFormat)
	RegisterRule(SectionKeyDataFormat, obj)
}
